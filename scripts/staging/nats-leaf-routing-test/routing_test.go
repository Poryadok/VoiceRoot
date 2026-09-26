//go:build linux

package routingtest

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

const (
	streamName   = "user_profile_projection"
	consumerName = "search-user-profile-projection-v1"
	consumerInfo = "$JS.API.CONSUMER.INFO." + streamName + "." + consumerName
	neighborInfo = "$JS.API.CONSUMER.INFO." + streamName + ".neighbor"
	inboxPrefix  = "_INBOX.voice.search"
)

func TestNonJetStreamLeafRequiresEmptyDefaultJSDomainMap(t *testing.T) {
	if server.VERSION != "2.12.12" {
		t.Fatalf("routing regression must run against NATS 2.12.12, got %s", server.VERSION)
	}
	for _, test := range []struct {
		name       string
		hubMap     bool
		leafMap    bool
		wantResult bool
	}{
		{name: "baseline_no_maps"},
		{name: "hub_map_only", hubMap: true},
		{name: "leaf_map_only", leafMap: true, wantResult: true},
		{name: "both_maps", hubMap: true, leafMap: true, wantResult: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newFixture(t, test.hubMap, test.leafMap)
			if !fixture.servers[0].JetStreamEnabled() {
				t.Fatal("hub JetStream must be enabled for the topology test")
			}
			if fixture.servers[1].JetStreamEnabled() {
				t.Fatal("leaf JetStream must remain disabled")
			}
			fixture.provisionConsumer(t)

			// The exact same user JWT is used for a direct hub control and the
			// remote leaf. This proves the consumer and its narrow API permission
			// are valid before testing leaf routing.
			hubClient := fixture.connect(t, fixture.hubURL, fixture.searchCreds)
			defer hubClient.Close()
			hubJS, err := hubClient.JetStream()
			if err != nil {
				t.Fatalf("direct hub JetStream client setup failed: %v", err)
			}
			if _, err := hubJS.ConsumerInfo(streamName, consumerName); err != nil {
				t.Fatalf("direct hub ConsumerInfo control failed: %v", err)
			}
			permissionErrors := make(chan error, 1)
			neighborClient := fixture.connectWithErrorHandler(t, fixture.hubURL, fixture.searchCreds, func(err error) {
				select {
				case permissionErrors <- err:
				default:
				}
			})
			defer neighborClient.Close()
			neighborJS, err := neighborClient.JetStream()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := neighborJS.ConsumerInfo(streamName, "neighbor"); err == nil {
				t.Fatal("neighboring ConsumerInfo unexpectedly succeeded")
			}
			select {
			case err := <-permissionErrors:
				if !strings.Contains(strings.ToLower(err.Error()), "permission") {
					t.Fatalf("neighboring ConsumerInfo error was not an ACL denial: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("exact-user ACL did not report its neighboring subject denial")
			}

			localClient := fixture.connect(t, fixture.leafURL, "")
			defer localClient.Close()
			response, err := localClient.Request(consumerInfo, nil, 2*time.Second)
			if test.wantResult {
				if err != nil {
					t.Fatalf("the leaf map should route the exact API reply through the non-JS leaf: %v", err)
				}
				var result consumerInfoResponse
				if err := json.Unmarshal(response.Data, &result); err != nil {
					t.Fatalf("decode local ConsumerInfo response: %v", err)
				}
				if result.Error != nil || result.Config.DurableName != consumerName {
					t.Fatalf("the leaf map should route the exact API reply through the non-JS leaf, response=%+v", result)
				}
				if _, err := localClient.Request(neighborInfo, nil, 2*time.Second); err == nil {
					t.Fatal("the local leaf unexpectedly routed the neighboring ConsumerInfo request")
				}
			} else if err != nil {
				if !strings.Contains(strings.ToLower(err.Error()), "no responders") {
					t.Fatalf("without the leaf map, the request should retain the NATS 2.12.12 no-responder deny, err=%v", err)
				}
			} else {
				var result consumerInfoResponse
				if err := json.Unmarshal(response.Data, &result); err != nil || result.Error == nil || result.Error.Code != 503 {
					t.Fatalf("without the leaf map, the request should retain the NATS 2.12.12 API 503 deny, response=%s", response.Data)
				}
			}
		})
	}
}

type fixture struct {
	root        string
	hubURL      string
	leafURL     string
	searchCreds string
	servers     []*server.Server
}

type consumerInfoResponse struct {
	Error *struct {
		Code        int    `json:"code"`
		Description string `json:"description"`
	} `json:"error"`
	Config struct {
		DurableName string `json:"durable_name"`
	} `json:"config"`
}

func newFixture(t *testing.T, hubMap, leafMap bool) *fixture {
	t.Helper()
	f := &fixture{root: t.TempDir()}
	operator, err := nkeys.CreateOperator()
	if err != nil {
		t.Fatal(err)
	}
	operatorPublic, err := operator.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	app, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	appPublic, err := app.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	system, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	systemPublic, err := system.PublicKey()
	if err != nil {
		t.Fatal(err)
	}

	operatorClaim := jwt.NewOperatorClaims(operatorPublic)
	operatorClaim.SystemAccount = systemPublic
	operatorJWT, err := operatorClaim.Encode(operator)
	if err != nil {
		t.Fatal(err)
	}
	appClaim := jwt.NewAccountClaims(appPublic)
	appClaim.Limits = jwt.OperatorLimits{
		NatsLimits:    jwt.NatsLimits{Subs: jwt.NoLimit, Data: jwt.NoLimit, Payload: jwt.NoLimit},
		AccountLimits: jwt.AccountLimits{Imports: jwt.NoLimit, Exports: jwt.NoLimit, WildcardExports: true, Conn: jwt.NoLimit, LeafNodeConn: jwt.NoLimit},
		JetStreamLimits: jwt.JetStreamLimits{
			MemoryStorage: jwt.NoLimit, DiskStorage: jwt.NoLimit, Streams: jwt.NoLimit,
			Consumer: jwt.NoLimit, MaxAckPending: jwt.NoLimit,
		},
	}
	appJWT, err := appClaim.Encode(operator)
	if err != nil {
		t.Fatal(err)
	}
	systemJWT, err := jwt.NewAccountClaims(systemPublic).Encode(operator)
	if err != nil {
		t.Fatal(err)
	}
	searchCreds, err := makeCreds(app, true)
	if err != nil {
		t.Fatal(err)
	}
	adminCreds, err := makeCreds(app, false)
	if err != nil {
		t.Fatal(err)
	}
	f.searchCreds = filepath.Join(f.root, "search.creds")
	adminPath := filepath.Join(f.root, "admin.creds")
	writeFile(t, f.searchCreds, searchCreds, 0600)
	writeFile(t, adminPath, adminCreds, 0600)

	hubDomain := ""
	if hubMap {
		hubDomain = fmt.Sprintf("default_js_domain: { %q: \"\" }", appPublic)
	}
	hubConfig := fmt.Sprintf(`
listen: 127.0.0.1:-1
operator: %q
system_account: %q
resolver: MEMORY
resolver_preload: { %q: %q, %q: %q }
jetstream { store_dir: %q }
%s
leafnodes { listen: 127.0.0.1:-1 }
`, filepath.Join(f.root, "operator.jwt"), systemPublic, appPublic, appJWT, systemPublic, systemJWT, filepath.Join(f.root, "hub-store"), hubDomain)
	writeFile(t, filepath.Join(f.root, "operator.jwt"), []byte(operatorJWT), 0600)
	hubOptions := startServer(t, f, filepath.Join(f.root, "hub.conf"), hubConfig)
	f.hubURL = fmt.Sprintf("nats://%s:%d", hubOptions.Host, hubOptions.Port)
	leafDomain := ""
	if leafMap {
		leafDomain = `default_js_domain: { "$G": "" }`
	}
	leafConfig := fmt.Sprintf(`
listen: 127.0.0.1:-1
%s
leafnodes { remotes: [{ urls: ["nats-leaf://127.0.0.1:%d"], account: "$G", credentials: %q }] }
`, leafDomain, hubOptions.LeafNode.Port, f.searchCreds)
	leafOptions := startServer(t, f, filepath.Join(f.root, "leaf.conf"), leafConfig)
	f.leafURL = fmt.Sprintf("nats://%s:%d", leafOptions.Host, leafOptions.Port)
	waitForLeaf(t, f.servers[0], f.servers[1])
	return f
}

func makeCreds(account nkeys.KeyPair, narrow bool) ([]byte, error) {
	user, err := nkeys.CreateUser()
	if err != nil {
		return nil, err
	}
	userPublic, err := user.PublicKey()
	if err != nil {
		return nil, err
	}
	claim := jwt.NewUserClaims(userPublic)
	claim.Limits.Subs = jwt.NoLimit
	claim.Limits.Data = -1
	claim.Limits.Payload = -1
	if narrow {
		claim.Permissions.Pub.Allow.Add(consumerInfo)
		claim.Permissions.Sub.Allow.Add(inboxPrefix + ".>")
	} else {
		claim.Permissions.Pub.Allow.Add(">")
		claim.Permissions.Sub.Allow.Add(">")
	}
	userJWT, err := claim.Encode(account)
	if err != nil {
		return nil, err
	}
	seed, err := user.Seed()
	if err != nil {
		return nil, err
	}
	return jwt.FormatUserConfig(userJWT, seed)
}

func (f *fixture) provisionConsumer(t *testing.T) {
	t.Helper()
	admin, err := nats.Connect(f.hubURL, nats.UserCredentials(filepath.Join(f.root, "admin.creds")), nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("hub provisioning client connect failed: %v", err)
	}
	defer admin.Close()
	js, err := admin.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.AddStream(&nats.StreamConfig{Name: streamName, Subjects: []string{"profile.projection"}, Storage: nats.MemoryStorage}); err != nil {
		t.Fatalf("create test stream: %v", err)
	}
	if _, err := js.AddConsumer(streamName, &nats.ConsumerConfig{Durable: consumerName, FilterSubject: "profile.projection", AckPolicy: nats.AckExplicitPolicy}); err != nil {
		t.Fatalf("create test consumer: %v", err)
	}
	if _, err := js.AddConsumer(streamName, &nats.ConsumerConfig{Durable: "neighbor", FilterSubject: "profile.projection", AckPolicy: nats.AckExplicitPolicy}); err != nil {
		t.Fatalf("create neighboring control consumer: %v", err)
	}
	if _, err := js.ConsumerInfo(streamName, "neighbor"); err != nil {
		t.Fatalf("verify neighboring control consumer exists: %v", err)
	}
}

func (f *fixture) connect(t *testing.T, url, credsPath string) *nats.Conn {
	return f.connectWithErrorHandler(t, url, credsPath, nil)
}

func (f *fixture) connectWithErrorHandler(t *testing.T, url, credsPath string, onError func(error)) *nats.Conn {
	t.Helper()
	options := []nats.Option{nats.Timeout(2 * time.Second), nats.CustomInboxPrefix(inboxPrefix)}
	if onError != nil {
		options = append(options, nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) { onError(err) }))
	}
	if credsPath != "" {
		options = append(options, nats.UserCredentials(credsPath))
	}
	conn, err := nats.Connect(url, options...)
	if err != nil {
		t.Fatalf("NATS connect failed: %v", err)
	}
	return conn
}

func startServer(t *testing.T, f *fixture, path, config string) *server.Options {
	t.Helper()
	writeFile(t, path, []byte(config), 0600)
	options, err := server.ProcessConfigFile(path)
	if err != nil {
		t.Fatalf("process NATS config: %v", err)
	}
	options.NoLog = true
	options.Debug = false
	options.Trace = false
	srv, err := server.NewServer(options)
	if err != nil {
		t.Fatalf("construct NATS server: %v", err)
	}
	f.servers = append(f.servers, srv)
	srv.Start()
	t.Cleanup(func() {
		srv.Shutdown()
		srv.WaitForShutdown()
	})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", options.Host, options.Port), 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return options
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("NATS server did not open its client port")
	return nil
}

func waitForLeaf(t *testing.T, hub, leaf *server.Server) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if hub.NumLeafNodes() > 0 && leaf.NumLeafNodes() > 0 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("NATS leaf link did not become active (hub_links=%d leaf_links=%d)", hub.NumLeafNodes(), leaf.NumLeafNodes())
}

func writeFile(t *testing.T, path string, contents []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, contents, mode); err != nil {
		t.Fatal(err)
	}
}
