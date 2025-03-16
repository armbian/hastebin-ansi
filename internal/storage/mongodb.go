package storage

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoDBStorage struct {
	db         *mongo.Database
	collection *mongo.Collection
	expiration time.Duration
}

type item struct {
	ObjectID   any       `json:"_id,omitempty" bson:"_id,omitempty"`
	Key        string    `json:"key" bson:"key"`
	Value      []byte    `json:"value" bson:"value"`
	Expiration time.Time `json:"expiration,omitempty" bson:"expiration,omitempty"`
}

func NewMongoDBStorage(host string, port int, username string, password string, database string, expiration time.Duration) *MongoDBStorage {
	ctx := context.Background()

	dsn := "mongodb://"
	if username != "" {
		dsn += url.QueryEscape(username)
	}

	if password != "" {
		dsn += ":" + url.QueryEscape(password)
	}

	if username != "" || password != "" {
		dsn += "@"
	}

	dsn += host + ":" + strconv.Itoa(port)

	client, err := mongo.Connect(options.Client().ApplyURI(dsn))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to MongoDB")
	}

	// Check if connection is established
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to MongoDB")
	}

	db := client.Database(database)

	command := bson.M{"create": "entries"}
	var result bson.M
	if err := db.RunCommand(context.Background(), command).Decode(&result); err != nil {
		panic(err)
	}

	// Create collection if not exists
	collection := db.Collection("entries")

	indexModel := mongo.IndexModel{
		Keys:    bson.M{"expiration": 1},
		Options: options.Index().SetExpireAfterSeconds(0),
	}

	if _, err := collection.Indexes().CreateOne(ctx, indexModel); err != nil {
		panic(err)
	}

	keyIndexModel := mongo.IndexModel{
		Keys:    bson.M{"key": 1},
		Options: options.Index().SetUnique(true),
	}

	if _, err := collection.Indexes().CreateOne(ctx, keyIndexModel); err != nil {
		panic(err)
	}

	return &MongoDBStorage{db: db, collection: collection, expiration: expiration}
}

func (s *MongoDBStorage) Set(key string, value string, skip_expiration bool) error {
	ctx := context.Background()

	// Convert value to []byte
	valueBytes := []byte(value)

	expiration := time.Now().Add(s.expiration)
	if skip_expiration {
		expiration = time.Time{}
	}

	// Create item
	i := item{
		Key:        key,
		Value:      valueBytes,
		Expiration: expiration,
	}

	// Insert item
	if _, err := s.collection.InsertOne(ctx, i); err != nil {
		return err
	}

	return nil
}

func (s *MongoDBStorage) Get(key string, skip_expiration bool) (string, error) {
	ctx := context.Background()

	// Find item
	filter := bson.M{"key": key}
	var i item
	if err := s.collection.FindOne(ctx, filter).Decode(&i); err != nil {
		return "", err
	}

	if !i.Expiration.IsZero() && i.Expiration.Unix() <= time.Now().Unix() {
		_, err := s.collection.DeleteOne(ctx, filter)
		if err != nil {
			return "", err
		}

		return "", ErrNotFound
	}

	// Update expiration
	if !skip_expiration {
		i.Expiration = time.Now().Add(s.expiration)
		update := bson.M{"$set": bson.M{"expiration": i.Expiration}}
		if _, err := s.collection.UpdateOne(ctx, filter, update); err != nil {
			return "", err
		}

	}

	return string(i.Value), nil
}

func (s *MongoDBStorage) Close() error {
	return s.db.Client().Disconnect(context.Background())
}
