package orm

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDBConf struct {
	DataBase string `yaml:"database"`
	Addr     string `yaml:"addr"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func InitMongoDbClient(dbConf MongoDBConf) (*mongo.Database, error) {
	opts := options.Client()
	ctx := context.TODO()
	schema := "mongodb"
	auth := ""
	if len(dbConf.Username) > 0 {
		auth = fmt.Sprintf("%s:%s", dbConf.Username, dbConf.Password)
	}
	uri := fmt.Sprintf("%s:%s//%s", schema, auth, dbConf.Addr)
	opts.ApplyURI(uri)
	opts.SetMaxPoolSize(50)

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}
	clientDB := client.Database(dbConf.DataBase)
	return clientDB, err
}
