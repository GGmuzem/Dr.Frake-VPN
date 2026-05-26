package agent

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"strings"
	"testing"
)

type fakeRunner struct {
	outputs map[string][]byte
	calls   []string
}

func (r *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, key)
	return r.outputs[key], nil
}

func TestSnapshotPacksAllowlistedDockerConfigs(t *testing.T) {
	runner := &fakeRunner{outputs: map[string][]byte{}}
	runner.outputs[`docker exec amnezia-xray sh -lc cat /opt/amnezia/xray/server.json 2>/dev/null || cat /opt/fblink/xray/server.json 2>/dev/null`] = []byte(`{"inbounds":[{"port":443}]}`)
	runner.outputs[`docker exec amnezia-xray sh -lc cat /opt/amnezia/xray/xray_public.key 2>/dev/null || cat /opt/fblink/xray/xray_public.key 2>/dev/null`] = []byte("public-key\n")
	runner.outputs[`docker exec amnezia-xray sh -lc cat /opt/amnezia/xray/xray_short_id.key 2>/dev/null || cat /opt/fblink/xray/xray_short_id.key 2>/dev/null`] = []byte("short-id\n")
	runner.outputs[`docker exec amnezia-xray sh -lc cat /opt/amnezia/xray/xray_uuid.key 2>/dev/null || cat /opt/fblink/xray/xray_uuid.key 2>/dev/null`] = []byte("uuid\n")
	runner.outputs[`docker exec amnezia-awg2 sh -lc cat /opt/amnezia/awg/awg0.conf 2>/dev/null || cat /opt/fblink/awg/awg0.conf 2>/dev/null`] = []byte("[Interface]\nPrivateKey = hidden\n")
	runner.outputs[`docker exec pihole sh -lc test -f /etc/pihole/gravity.db && echo gravity-db-present || true`] = []byte("gravity-db-present\n")

	packed, manifest, err := NewSnapshotBuilder(SnapshotConfig{
		NodeID:          "node-1",
		DockerBin:       "docker",
		XrayContainer:   "amnezia-xray",
		AWGContainer:    "amnezia-awg2",
		AWGInterface:    "awg0",
		PiHoleContainer: "pihole",
	}, runner).Pack(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ContentHash == "" {
		t.Fatal("expected manifest content hash")
	}

	files := readTarGz(t, packed)
	for _, name := range []string{
		"manifest.json",
		"xray/server.json",
		"xray/xray_public.key",
		"xray/xray_short_id.key",
		"xray/xray_uuid.key",
		"awg/awg0.conf",
		"pihole/metadata.txt",
	} {
		if _, ok := files[name]; !ok {
			t.Fatalf("expected %s in snapshot", name)
		}
	}
}

func readTarGz(t *testing.T, packed []byte) map[string]string {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	files := map[string]string{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return files
		}
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		files[h.Name] = string(body)
	}
}
