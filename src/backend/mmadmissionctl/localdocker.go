package mmadmissionctl

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	w "voice/backend/pkg/mmadmissionwire"
)

// ownedDocker cannot adopt resources. A persisted creation intent from this
// journal and its unique nonce are prerequisites to every platform read/mutation.
// It is deliberately not an installed Kubernetes/Compose driver.
type ownedDocker struct {
	j     *Journal
	scope w.ID
	http  *http.Client
}
type ownedObject struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	ID          string `json:"id"`
	Nonce       w.ID   `json:"nonce"`
	RequestHash w.Hash `json:"request_hash"`
}

const ownLabel = "voice.mm.local.scope"
const nonceLabel = "voice.mm.local.creation"
const requestLabel = "voice.mm.local.request"

func newOwnedDocker(j *Journal, scope w.ID) (*ownedDocker, error) {
	if scope == (w.ID{}) {
		return nil, ErrConflict
	}
	_, e := j.db.Exec(`CREATE TABLE IF NOT EXISTS local_objects(name TEXT PRIMARY KEY,kind TEXT NOT NULL,nonce TEXT NOT NULL,request_hash BLOB NOT NULL,request BLOB NOT NULL,object_id TEXT NOT NULL)`)
	if e != nil {
		return nil, e
	}
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "unix", "/var/run/docker.sock")
	}}
	return &ownedDocker{j: j, scope: scope, http: &http.Client{Transport: tr, Timeout: 30 * time.Second}}, nil
}
func (d *ownedDocker) call(ctx context.Context, method, path string, input any, out any) error {
	var body io.Reader
	if input != nil {
		raw, e := json.Marshal(input)
		if e != nil {
			return e
		}
		body = bytes.NewReader(raw)
	}
	r, e := http.NewRequestWithContext(ctx, method, "http://docker/v1.47"+path, body)
	if e != nil {
		return e
	}
	r.Header.Set("Content-Type", "application/json")
	res, e := d.http.Do(r)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if e != nil {
		return e
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("owned Docker %s status %d", method, res.StatusCode)
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}
func (d *ownedDocker) name(suffix string) string {
	return "mm-local-" + strings.ReplaceAll(d.scope.String(), "-", "") + "-" + suffix
}
func (d *ownedDocker) create(ctx context.Context, kind, suffix string, config map[string]any) (ownedObject, error) {
	name := d.name(suffix)
	var o ownedObject
	o.Kind = kind
	o.Name = name
	var nonce string
	var hash, request []byte
	e := d.j.db.QueryRowContext(ctx, `SELECT nonce,request_hash,request,object_id FROM local_objects WHERE name=? AND kind=?`, name, kind).Scan(&nonce, &hash, &request, &o.ID)
	if e == nil {
		if len(hash) != 32 {
			return o, ErrPending
		}
		copy(o.RequestHash[:], hash)
		o.Nonce, e = w.ParseID(nonce)
		if e != nil {
			return o, e
		}
		if o.ID == "" {
			return d.recoverCreation(ctx, o)
		}
		_, e = d.inspect(ctx, o)
		return o, e
	}
	if e != sql.ErrNoRows {
		return o, e
	}
	o.Nonce = freshID()
	labels := map[string]string{ownLabel: d.scope.String(), nonceLabel: o.Nonce.String()}
	config["Labels"] = labels
	if kind == "container" {
		delete(config, "Labels")
		config["Labels"] = labels
	}
	request, e = json.Marshal(config)
	if e != nil {
		return o, e
	}
	o.RequestHash = sha256.Sum256(request)
	labels[requestLabel] = o.RequestHash.String()
	request, e = json.Marshal(config)
	if e != nil {
		return o, e
	}
	if _, e = d.j.db.ExecContext(ctx, `INSERT INTO local_objects(name,kind,nonce,request_hash,request,object_id)VALUES(?,?,?,?,?,'')`, name, kind, o.Nonce.String(), o.RequestHash[:], request); e != nil {
		return o, e
	}
	path := ""
	switch kind {
	case "network":
		path = "/networks/create"
		config["Name"] = name
	case "volume":
		path = "/volumes/create"
		config["Name"] = name
	case "container":
		path = "/containers/create?name=" + url.QueryEscape(name)
	default:
		return o, ErrConflict
	}
	var result struct {
		ID   string `json:"Id"`
		Name string `json:"Name"`
	}
	if e = d.call(ctx, http.MethodPost, path, config, &result); e != nil {
		return o, ErrPending
	}
	o.ID = result.ID
	if kind == "volume" {
		o.ID = result.Name
	}
	if o.ID == "" {
		return o, ErrPending
	}
	if _, e = d.j.db.ExecContext(ctx, `UPDATE local_objects SET object_id=? WHERE name=? AND object_id=''`, o.ID, name); e != nil {
		return o, e
	}
	_, e = d.inspect(ctx, o)
	return o, e
}
func (d *ownedDocker) recoverCreation(ctx context.Context, o ownedObject) (ownedObject, error) {
	// Lost create ACK is classified by the recorded exact name/nonce/request, never
	// by a label search, guessed matching resource or blind create retry.
	candidate := o
	candidate.ID = o.Name
	raw, e := d.inspect(ctx, candidate)
	if e != nil {
		return o, ErrPending
	}
	var v struct {
		ID   string `json:"Id"`
		Name string `json:"Name"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return o, ErrPending
	}
	if o.Kind == "volume" {
		o.ID = v.Name
	} else {
		o.ID = v.ID
	}
	if o.ID == "" {
		return o, ErrPending
	}
	_, e = d.j.db.ExecContext(ctx, `UPDATE local_objects SET object_id=? WHERE name=? AND object_id=''`, o.ID, o.Name)
	return o, e
}
func (d *ownedDocker) inspect(ctx context.Context, o ownedObject) ([]byte, error) {
	if o.Name == "" || !strings.HasPrefix(o.Name, d.name("")) || o.ID == "" || o.Nonce == (w.ID{}) || o.RequestHash == (w.Hash{}) {
		return nil, ErrConflict
	}
	var raw json.RawMessage
	path := ""
	switch o.Kind {
	case "container":
		path = "/containers/" + url.PathEscape(o.ID) + "/json"
	case "network":
		path = "/networks/" + url.PathEscape(o.ID)
	case "volume":
		path = "/volumes/" + url.PathEscape(o.ID)
	default:
		return nil, ErrConflict
	}
	if e := d.call(ctx, http.MethodGet, path, nil, &raw); e != nil {
		return nil, e
	}
	var inspected struct {
		Labels map[string]string
		Config struct{ Labels map[string]string }
		ID     string `json:"Id"`
		Name   string
	}
	if json.Unmarshal(raw, &inspected) != nil {
		return nil, ErrPending
	}
	labels := inspected.Labels
	if o.Kind == "container" {
		labels = inspected.Config.Labels
	}
	if labels[ownLabel] != d.scope.String() || labels[nonceLabel] != o.Nonce.String() || labels[requestLabel] != o.RequestHash.String() {
		return nil, ErrConflict
	}
	return raw, nil
}
func (d *ownedDocker) start(ctx context.Context, o ownedObject) error {
	raw, e := d.inspect(ctx, o)
	if e != nil {
		return e
	}
	var v struct{ State struct{ Running bool } }
	if json.Unmarshal(raw, &v) != nil {
		return ErrPending
	}
	if v.State.Running {
		return nil
	}
	return d.call(ctx, http.MethodPost, "/containers/"+url.PathEscape(o.ID)+"/start", nil, nil)
}
func (d *ownedDocker) objects(ctx context.Context) ([]ownedObject, error) {
	rows, e := d.j.db.QueryContext(ctx, `SELECT name,kind,nonce,request_hash,object_id FROM local_objects ORDER BY name`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []ownedObject
	for rows.Next() {
		var o ownedObject
		var nonce string
		var h []byte
		if rows.Scan(&o.Name, &o.Kind, &nonce, &h, &o.ID) != nil || len(h) != 32 {
			return nil, ErrPending
		}
		o.Nonce, e = w.ParseID(nonce)
		if e != nil {
			return nil, e
		}
		copy(o.RequestHash[:], h)
		out = append(out, o)
	}
	return out, rows.Err()
}
func (d *ownedDocker) closeOwned(ctx context.Context) error {
	objects, e := d.objects(ctx)
	if e != nil {
		return e
	}
	for _, o := range objects {
		if o.Kind != "container" {
			continue
		}
		raw, err := d.inspect(ctx, o)
		if err != nil {
			return err
		}
		var state struct{ State struct{ Running bool } }
		if json.Unmarshal(raw, &state) != nil {
			return ErrPending
		}
		if !state.State.Running {
			continue
		}
		if _, e = d.inspect(ctx, o); e != nil {
			return e
		}
		if e = d.call(ctx, http.MethodPost, "/containers/"+url.PathEscape(o.ID)+"/stop?t=5", nil, nil); e != nil {
			return e
		}
	}
	return nil
}
func (d *ownedDocker) cleanup(ctx context.Context, self string) error {
	if e := d.closeOwned(ctx); e != nil {
		return e
	}
	objects, e := d.objects(ctx)
	if e != nil {
		return e
	}
	for _, kind := range []string{"container", "network", "volume"} {
		for _, o := range objects {
			if o.Kind != kind {
				continue
			}
			if _, e = d.inspect(ctx, o); e != nil {
				return e
			}
			path := ""
			switch kind {
			case "container":
				path = "/containers/" + url.PathEscape(o.ID)
			case "network":
				if self != "" {
					_ = d.call(ctx, http.MethodPost, "/networks/"+url.PathEscape(o.ID)+"/disconnect", map[string]any{"Container": self, "Force": false}, nil)
				}
				path = "/networks/" + url.PathEscape(o.ID)
			case "volume":
				path = "/volumes/" + url.PathEscape(o.ID)
			}
			if e = d.call(ctx, http.MethodDelete, path, nil, nil); e != nil {
				return e
			}
		}
	}
	return nil
}
func (d *ownedDocker) verifyUniverse(ctx context.Context, network ownedObject, expected []string) error {
	raw, e := d.inspect(ctx, network)
	if e != nil {
		return e
	}
	var n struct{ Containers map[string]json.RawMessage }
	if json.Unmarshal(raw, &n) != nil {
		return ErrPending
	}
	allow := map[string]bool{}
	for _, id := range expected {
		allow[id] = true
	}
	if len(n.Containers) != len(allow) {
		return ErrPending
	}
	for id := range n.Containers {
		if !allow[id] {
			return ErrPending
		}
	}
	return nil
}
