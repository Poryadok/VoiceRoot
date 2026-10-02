"""Apply the Voice adapter to hash-pinned upstream source, failing on drift."""
import pathlib
import shutil
import sys

server, protocol, authority, adapter = map(pathlib.Path, sys.argv[1:])

def replace(path, before, after):
    text = path.read_text(encoding='utf-8')
    if text.count(before) != 1:
        raise SystemExit('pinned source anchor drift: ' + str(path))
    path.write_text(text.replace(before, after), encoding='utf-8', newline='\n')

replace(protocol / 'auth/grants.go', 'type ClaimGrants struct {',
        'type ClaimGrants struct {\n\t// Private Voice authority claim; never copied to participant metadata/attributes.\n\tVoiceMediaGrant string `json:"voice_media_grant,omitempty"`')

replace(server / 'cmd/server/main.go', '\tif err := app.Run(os.Args); err != nil {\n\t\tfmt.Println(err)\n\t}',
        '\tif err := app.Run(os.Args); err != nil {\n\t\tfmt.Fprintln(os.Stderr, err)\n\t\tos.Exit(1)\n\t}')

node = server / 'pkg/routing/node.go'
replace(node, 'import (\n', 'import (\n\t"fmt"\n\t"os"\n\t"github.com/google/uuid"\n')
replace(node, '\tnodeID := guid.New(utils.NodePrefix)', '''	nodeID := guid.New(utils.NodePrefix)
	if os.Getenv("VOICE_SFU_AUTHORITY_DIR") != "" {
		nodeID = os.Getenv("VOICE_NODE_ID")
		id, err := uuid.Parse(nodeID)
		if err != nil || id == uuid.Nil || id.String() != nodeID {
			return nil, fmt.Errorf("Voice SFU requires its registered canonical node UUID")
		}
	}''')

room = server / 'pkg/service/roommanager.go'
replace(room, 'import (\n', 'import (\n\t"github.com/livekit/livekit-server/pkg/voiceauthority/mediaauthority"\n')
replace(room, 'type RoomManager struct {', 'type RoomManager struct {\n\tvoiceAuthority *voiceAuthorityRuntime')
replace(room, '\tr.roomManagerServer, err = rpc.NewTypedRoomManagerServer', '''	r.voiceAuthority, err = newVoiceAuthority(r)
	if err != nil {
		return nil, err
	}
	r.roomManagerServer, err = rpc.NewTypedRoomManagerServer''')
replace(room, 'func (r *RoomManager) Stop() {', '''func (r *RoomManager) Stop() {
	if r.voiceAuthority != nil {
		r.voiceAuthority.stop()
	}''')
replace(room, '\tsessionStartTime := time.Now()', '''	sessionStartTime := time.Now()
	var voiceAdmission mediaauthority.Admission
	if r.voiceAuthority != nil && pi.Identity != "" {
		if pi.CreateRoom == nil || pi.Grants == nil {
			return mediaauthority.ErrDenied
		}
		var err error
		voiceAdmission, err = r.voiceAuthority.registry.Admit(pi.Grants.VoiceMediaGrant, pi.CreateRoom.Name, string(pi.Identity), time.Now())
		if err != nil {
			return err
		}
		if pi.Grants.Video == nil || (pi.Grants.Video.GetCanPublish() && !voiceAdmission.CanPublish()) || pi.Grants.Video.GetCanPublishData() {
			return mediaauthority.ErrDenied
		}
	}''')
replace(room, '\t\t\tif err = room.ResumeParticipant(', '''			if r.voiceAuthority != nil {
				if r.voiceAuthority.registry.Check(voiceAdmission, time.Now()) != nil {
					return mediaauthority.ErrDenied
				}
				r.voiceAuthority.remember(participant.ID(), voiceAdmission)
			}
			if err = room.ResumeParticipant(''')
replace(room, '\tif err = room.Join(participant, requestSource, &opts, iceServers); err != nil {', '''	if r.voiceAuthority != nil {
		if r.voiceAuthority.registry.Check(voiceAdmission, time.Now()) != nil {
			_ = participant.Close(true, types.ParticipantCloseReasonVerifyFailed, false)
			return mediaauthority.ErrDenied
		}
		r.voiceAuthority.remember(participant.ID(), voiceAdmission)
	}
	if err = room.Join(participant, requestSource, &opts, iceServers); err != nil {''')
replace(room, '\tjwt, err := token.ToJWT()', '''	// Refresh preserves the original admission credential without renewing it.
	token.GetGrants().VoiceMediaGrant = grants.VoiceMediaGrant
	jwt, err := token.ToJWT()''')
shutil.copy2(adapter, server / 'pkg/service/voice_authority.go')

destination = server / 'pkg/voiceauthority'
for package in ['protocol', 'nodecache', 'mediaauthority']:
    target = destination / package
    target.mkdir(parents=True, exist_ok=True)
    for source in (authority / package).glob('*.go'):
        text = source.read_text(encoding='utf-8').replace('voice/backend/federation/', 'github.com/livekit/livekit-server/pkg/voiceauthority/')
        (target / source.name).write_text(text, encoding='utf-8', newline='\n')
