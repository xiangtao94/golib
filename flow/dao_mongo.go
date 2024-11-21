package flow

import (
	"go.mongodb.org/mongo-driver/mongo"
)

var DefaultMongoDBClient *mongo.Database

type MongoDao struct {
	Layer
	db             *mongo.Database
	defaultDB      *mongo.Database
	collectionName string
}

func (m *MongoDao) GetDB() *mongo.Collection {
	var dbB *mongo.Database
	if m.db != nil {
		dbB = m.db
	} else if m.defaultDB != nil {
		dbB = m.defaultDB
	} else if DefaultMongoDBClient != nil {
		dbB = DefaultMongoDBClient
	}
	if dbB != nil {
		return dbB.Collection(m.GetCollection())
	}
	return nil
}

func (m *MongoDao) SetDB(db *mongo.Database) {
	m.db = db
}

func (m *MongoDao) SetDefaultDB(db *mongo.Database) {
	m.defaultDB = db
}

func (m *MongoDao) SetCollection(collectionName string) {
	m.collectionName = collectionName
}

func (m *MongoDao) GetCollection() string {
	return m.collectionName
}
