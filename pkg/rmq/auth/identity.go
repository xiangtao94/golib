package auth

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"hash"
	"strconv"
)

const (
	ClientTypeProducer = ClientType(iota + 1)
	ClientTypeConsumer
)

const (
	skHashToken = "rocketmqsalt-m2hYy4ubPjKQNYKu"
)

type Identity struct {
	name     string
	group    string
	clntType ClientType
}

func NewIdentity(name string, group string, clntType ClientType) *Identity {
	return &Identity{
		name:     name,
		group:    group,
		clntType: clntType,
	}
}

func (id *Identity) Name() string     { return id.name }
func (id *Identity) Group() string    { return id.group }
func (id *Identity) Type() ClientType { return id.clntType }

/*
format:

	accessKey     = <name> + "|" + <clientType> + "|" + <group>
	securityToken = hmac(accessKey, skHashToken)
*/
func (id *Identity) Credential() (accessKey, securityKey string) {
	akBuff := []byte(id.name + "|" + strconv.Itoa(int(id.clntType)) + "|" + id.group)

	accessKey = base64.URLEncoding.EncodeToString(akBuff)
	securityKey = GetSecurityToken(string(accessKey))
	return
}

func (id *Identity) String() string {
	return fmt.Sprintf("Identity[Name: %s, Group: %s, Type: %v]", id.name, id.group, id.clntType)
}

func GetSecurityToken(accessKey string) string {
	rawAK, err := base64.URLEncoding.DecodeString(accessKey)
	if err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(hashAK(rawAK))
}

func hashAK(input []byte) []byte {
	mac := hmac.New(func() hash.Hash {
		return sha1.New()
	}, []byte(skHashToken))

	mac.Write(input)
	return mac.Sum(nil)
}

type ClientType int

func (ct ClientType) String() string {
	switch ct {
	case ClientTypeProducer:
		return "Producer"
	case ClientTypeConsumer:
		return "Consumer"
	default:
		return fmt.Sprintf("Unknown/%d", ct)
	}
}
