package mongodb

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PowInfo struct {
	ID    string
	Token string
}

const (
	key      = "pow_info"
	idKey    = "id"
	tokenKey = "token"
)

func encodePowInfo(data bson.M, powInfo *PowInfo) {
	if powInfo != nil {
		data[key] = bson.M{
			idKey:    powInfo.ID,
			tokenKey: powInfo.Token,
		}
	}
}

func decodePowInfo(raw primitive.M) *PowInfo {
	var powInfo *PowInfo
	if v, ok := raw[key]; ok {
		powInfo = &PowInfo{}
		raw := v.(bson.M)
		if v, ok := raw[idKey]; ok {
			powInfo.ID = v.(string)
		}
		if v, ok := raw[tokenKey]; ok {
			powInfo.Token = v.(string)
		}
	}
	return powInfo
}
