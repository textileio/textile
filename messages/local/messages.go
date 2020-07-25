package local

//
//import (
//	"context"
//
//	"github.com/textileio/go-threads/core/thread"
//	"github.com/textileio/textile/api/users/client"
//	pb "github.com/textileio/textile/api/users/pb"
//)
//
//type Messages struct {
//	c *client.Client
//}
//
//// SendMessage sends the message body to a recipient.
//func (m *Messages) SendMessage(ctx context.Context, from thread.Identity, to thread.PubKey, body []byte) (string, error) {
//	cbody, err := to.Encrypt(body)
//	if err != nil {
//		return nil, err
//	}
//	sig, err := from.Sign(ctx, cbody)
//	if err != nil {
//		return nil, err
//	}
//	return m.c.SendMessage(ctx, &pb.SendMessageRequest{
//		To:        to.String(),
//		Body:      cbody,
//		Signature: sig,
//	})
//}
