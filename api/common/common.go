package common

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	mbase "github.com/multiformats/go-multibase"
	"github.com/textileio/go-threads/core/thread"
)

type ctxKey string

// NewSessionContext adds a session to a context.
func NewSessionContext(ctx context.Context, session string) context.Context {
	if session == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey("session"), session)
}

// SessionFromContext returns a session from a context.
func SessionFromContext(ctx context.Context) (string, bool) {
	session, ok := ctx.Value(ctxKey("session")).(string)
	return session, ok
}

// SessionFromMD returns a session from context metadata.
func SessionFromMD(ctx context.Context) (session string, ok bool) {
	session = metautils.ExtractIncoming(ctx).Get("x-textile-session")
	if session != "" {
		ok = true
	}
	return
}

// NewOrgSlugContext adds an org name to a context.
func NewOrgSlugContext(ctx context.Context, slug string) context.Context {
	if slug == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey("orgSlug"), slug)
}

// OrgSlugFromContext returns an org name from a context.
func OrgSlugFromContext(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(ctxKey("orgSlug")).(string)
	return name, ok
}

// OrgSlugFromMD returns an org name from context metadata.
func OrgSlugFromMD(ctx context.Context) (slug string, ok bool) {
	slug = metautils.ExtractIncoming(ctx).Get("x-textile-org")
	if slug != "" {
		ok = true
	}
	return
}

// NewAPIKeyContext adds an API key to a context.
func NewAPIKeyContext(ctx context.Context, key string) context.Context {
	if key == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey("apiKey"), key)
}

// APIKeyFromContext returns an API key from a context.
func APIKeyFromContext(ctx context.Context) (string, bool) {
	key, ok := ctx.Value(ctxKey("apiKey")).(string)
	return key, ok
}

// APIKeyFromMD returns an API key from context metadata.
func APIKeyFromMD(ctx context.Context) (key string, ok bool) {
	key = metautils.ExtractIncoming(ctx).Get("x-textile-api-key")
	if key != "" {
		ok = true
	}
	return
}

// CreateAPISigContext creates an HMAC signature and adds it to a context,
// with secret as the key and SHA256 as the hash algorithm.
// An RFC 3339 date string is used as the message.
// Date must be sometime in the future. Dates closer to now are more secure.
func CreateAPISigContext(ctx context.Context, date time.Time, secret string) (context.Context, error) {
	_, sec, err := mbase.Decode(secret)
	if err != nil {
		return ctx, err
	}
	hash := hmac.New(sha256.New, sec)
	msg := date.Format(time.RFC3339)
	_, err = hash.Write([]byte(msg))
	if err != nil {
		return ctx, err
	}
	ctx = context.WithValue(ctx, ctxKey("apiSigMsg"), msg)
	ctx = context.WithValue(ctx, ctxKey("apiSig"), hash.Sum(nil))
	return ctx, nil
}

// NewAPISigContext adds an API key signature to a context.
func NewAPISigContext(ctx context.Context, msg string, sig []byte) context.Context {
	if sig == nil {
		return ctx
	}
	ctx = context.WithValue(ctx, ctxKey("apiSigMsg"), msg)
	ctx = context.WithValue(ctx, ctxKey("apiSig"), sig)
	return ctx
}

// APISigFromContext returns a message and signature from a context.
func APISigFromContext(ctx context.Context) (msg string, sig []byte, ok bool) {
	sig, ok = ctx.Value(ctxKey("apiSig")).([]byte)
	if !ok {
		return
	}
	msg, ok = ctx.Value(ctxKey("apiSigMsg")).(string)
	if !ok {
		return
	}
	return msg, sig, ok
}

// APISigFromMD returns a message and signature from context metadata.
func APISigFromMD(ctx context.Context) (msg string, sig []byte, ok bool) {
	str := metautils.ExtractIncoming(ctx).Get("x-textile-api-sig")
	if str == "" {
		return
	}
	var err error
	_, sig, err = mbase.Decode(str)
	if err != nil {
		return
	}
	msg = metautils.ExtractIncoming(ctx).Get("x-textile-api-sig-msg")
	if msg == "" {
		return
	}
	return msg, sig, true
}

// ValidateAPISigContext re-computes the hash from a context using secret as key.
// This method returns true only if the hashes are equal and the message is a
// valid RFC 3339 date string sometime in the future.
func ValidateAPISigContext(ctx context.Context, secret string) bool {
	msg, sig, ok := APISigFromContext(ctx)
	if !ok {
		return false
	}
	_, sec, err := mbase.Decode(secret)
	if err != nil {
		return false
	}
	date, err := time.Parse(time.RFC3339, msg)
	if err != nil {
		return false
	}
	if date.Before(time.Now()) {
		return false
	}
	hash := hmac.New(sha256.New, sec)
	_, err = hash.Write([]byte(msg))
	if err != nil {
		return false
	}
	return hmac.Equal(sig, hash.Sum(nil))
}

// NewThreadIDContext adds a thread ID to a context.
func NewThreadIDContext(ctx context.Context, id thread.ID) context.Context {
	if !id.Defined() {
		return ctx
	}
	return context.WithValue(ctx, ctxKey("threadID"), id)
}

// ThreadIDFromContext returns a thread ID from a context.
func ThreadIDFromContext(ctx context.Context) (thread.ID, bool) {
	id, ok := ctx.Value(ctxKey("threadID")).(thread.ID)
	return id, ok
}

// ThreadIDFromMD returns a thread ID from context metadata.
func ThreadIDFromMD(ctx context.Context) (id thread.ID, ok bool) {
	str := metautils.ExtractIncoming(ctx).Get("x-textile-thread")
	if str == "" {
		return
	}
	var err error
	id, err = thread.Decode(str)
	if err != nil {
		return id, false
	}
	return id, true
}

// NewThreadNameContext adds a thread name to a context,
// which is used to name threads and dbs during creation.
// Thread names may be useful for some app logic, but are not required.
func NewThreadNameContext(ctx context.Context, name string) context.Context {
	if name == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey("threadName"), name)
}

// ThreadNameFromContext returns a thread name from a context.
func ThreadNameFromContext(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(ctxKey("threadName")).(string)
	return name, ok
}

// ThreadNameFromMD returns a thread name from context metadata.
func ThreadNameFromMD(ctx context.Context) (name string, ok bool) {
	name = metautils.ExtractIncoming(ctx).Get("x-textile-thread-name")
	if name != "" {
		ok = true
	}
	return
}

// Credentials implements grpc.PerRPCCredentials.
type Credentials struct {
	Secure bool
}

func (c Credentials) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	md := map[string]string{}
	session, ok := SessionFromContext(ctx)
	if ok {
		md["x-textile-session"] = session
	}
	orgSlug, ok := OrgSlugFromContext(ctx)
	if ok {
		md["x-textile-org"] = orgSlug
	}
	apiKey, ok := APIKeyFromContext(ctx)
	if ok {
		md["x-textile-api-key"] = apiKey
	}
	apiSigMsg, apiSig, ok := APISigFromContext(ctx)
	if ok {
		var err error
		md["x-textile-api-sig"], err = mbase.Encode(mbase.Base32, apiSig)
		if err != nil {
			return nil, err
		}
		md["x-textile-api-sig-msg"] = apiSigMsg
	}
	threadID, ok := ThreadIDFromContext(ctx)
	if ok {
		md["x-textile-thread"] = threadID.String()
	}
	threadName, ok := ThreadNameFromContext(ctx)
	if ok {
		md["x-textile-thread-name"] = threadName
	}
	threadToken, ok := thread.TokenFromContext(ctx)
	if ok {
		md["authorization"] = "bearer " + string(threadToken)
	}
	return md, nil
}

func (c Credentials) RequireTransportSecurity() bool {
	return c.Secure
}
