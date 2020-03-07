package client_test

/*
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/google/uuid"
	threadsclient "github.com/textileio/go-threads/api/client"
	tutil "github.com/textileio/go-threads/util"
	c "github.com/textileio/textile/api/client"
	"github.com/textileio/textile/collections"
	"google.golang.org/grpc"
)

func TestThreadsClient_NewStore(t *testing.T) {
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

	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrThreadsApi)
	if err != nil {
		t.Fatal(err)
	}
	threads, err := threadsclient.NewClient(target, grpc.WithInsecure(),
		grpc.WithPerRPCCredentials(collections.TokenAuth{}))
	if err != nil {
		t.Fatal(err)
	}
	defer threads.Close()

	t.Run("test new store without client token", func(t *testing.T) {
		if _, err := threads.NewStore(context.Background()); err == nil {
			t.Fatal("test new store without client token should fail")
		}
	})

	t.Run("test new store with client token", func(t *testing.T) {
		storeID, err := threads.NewStore(collections.AuthCtx(context.Background(), session))
		if err != nil {
			t.Fatalf("new store with client token should succeed: %v", err)
		}
		t.Logf("client created store: %s", storeID)
	})
}

*/
