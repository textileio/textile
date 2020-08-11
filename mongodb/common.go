package mongodb

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FFSInfo struct {
	ID    string
	Token string
}

const (
	key      = "ffs_info"
	idKey    = "id"
	tokenKey = "token"
)

func encodeFFSInfo(data bson.M, ffsInfo *FFSInfo) {
	if ffsInfo != nil {
		data[key] = bson.M{
			idKey:    ffsInfo.ID,
			tokenKey: ffsInfo.Token,
		}
	}
}

func decodeFFSInfo(raw primitive.M) *FFSInfo {
	var ffsInfo *FFSInfo
	if v, ok := raw[key]; ok {
		ffsInfo = &FFSInfo{}
		raw := v.(bson.M)
		if v, ok := raw[idKey]; ok {
			ffsInfo.ID = v.(string)
		}
		if v, ok := raw[tokenKey]; ok {
			ffsInfo.Token = v.(string)
		}
	}
	return ffsInfo
}
