package client_test

/*
import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/google/uuid"
	crypto "github.com/libp2p/go-libp2p-core/crypto"
	threadscore "github.com/textileio/go-threads/core/service"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/crypto/symmetric"
	serviceclient "github.com/textileio/go-threads/service/api/client"
	tutil "github.com/textileio/go-threads/util"
	c "github.com/textileio/textile/api/client"
	"github.com/textileio/textile/collections"
	"google.golang.org/grpc"
)

func TestThreadsServiceClient_CreateThread(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	project, err := client.AddProject(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}
	token, err := client.AddToken(context.Background(), project.Name, c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}
	req, err := json.Marshal(&map[string]string{
		"token":     token.ID,
		"device_id": uuid.New().String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	url := fmt.Sprintf("%s/register", conf.AddrGatewayUrl)
	res, err := http.Post(url, "application/json", bytes.NewReader(req))
	if err != nil {
		t.Fatal(err)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	var data map[string]string
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatal(err)
	}
	session, ok := data["session_id"]
	if !ok {
		t.Fatalf("response body missing session id")
	}

	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrThreadsServiceApi)
	if err != nil {
		t.Fatal(err)
	}
	service, err := serviceclient.NewClient(target, grpc.WithInsecure(),
		grpc.WithPerRPCCredentials(collections.TokenAuth{}))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	id := thread.NewIDV1(thread.Raw, 32)
	fk, err := symmetric.CreateKey()
	if err != nil {
		t.Fatal(err)
	}
	_, pk, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test create thread without client token", func(t *testing.T) {
		if _, err := service.CreateThread(
			context.Background(), id, threadscore.FollowKey(fk), threadscore.LogKey(pk)); err == nil {
			t.Fatal("test create thread without client token should fail")
		}
	})

	t.Run("test create thread with client token", func(t *testing.T) {
		info, err := service.CreateThread(
			collections.AuthCtx(context.Background(), session), id, threadscore.FollowKey(fk), threadscore.LogKey(pk))
		if err != nil {
			t.Fatalf("create thread with client token should succeed: %v", err)
		}
		t.Logf("client created thread: %s", info.ID)
	})
}
*/
