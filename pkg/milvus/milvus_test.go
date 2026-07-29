package milvus

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/milvus-io/milvus/client/v2/entity"
	milvusindex "github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/stretchr/testify/require"
)

func TestNewMilvusClientContextRejectsNilContext(t *testing.T) {
	//lint:ignore SA1012 This test verifies that nil contexts are rejected.
	_, err := NewMilvusClientContext(nil, MilvusConf{})
	require.EqualError(t, err, "milvus: nil context")
}

func TestMilvusClientConnectionAlwaysHasADeadline(t *testing.T) {
	var remaining time.Duration
	factory := func(
		ctx context.Context,
		_ *milvusclient.ClientConfig,
	) (*milvusclient.Client, error) {
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		remaining = time.Until(deadline)
		return nil, errors.New("stop before network")
	}

	_, err := newMilvusClient(
		context.Background(),
		MilvusConf{ConnectTimeout: 250 * time.Millisecond},
		factory,
	)

	require.ErrorContains(t, err, "stop before network")
	require.Positive(t, remaining)
	require.LessOrEqual(t, remaining, 250*time.Millisecond)
}

func TestValidateVectors(t *testing.T) {
	tests := []struct {
		name    string
		vectors [][]float32
		wantDim int
		wantErr string
	}{
		{name: "empty", wantErr: "vectors must not be empty"},
		{name: "zero dimension", vectors: [][]float32{{}}, wantErr: "vector dimension must be greater than zero"},
		{
			name:    "inconsistent dimensions",
			vectors: [][]float32{{1, 2}, {3}},
			wantErr: "vector 1 has dimension 1, want 2",
		},
		{name: "valid", vectors: [][]float32{{1, 2}, {3, 4}}, wantDim: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dimension, err := validateVectors(tt.vectors)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantDim, dimension)
		})
	}
}

func TestNewIndexByTypeDoesNotMutateParams(t *testing.T) {
	params := map[string]string{"nlist": "1024"}

	index := NewIndexByType(milvusindex.IvfFlat, entity.L2, params)

	require.Equal(t, map[string]string{"nlist": "1024"}, params)
	require.Equal(t, "1024", index.Params()["nlist"])
	require.Equal(t, string(milvusindex.IvfFlat), index.Params()[milvusindex.IndexTypeKey])
	require.Equal(t, string(entity.L2), index.Params()[milvusindex.MetricTypeKey])
}

func TestCloseRejectsNilContextWithoutDriver(t *testing.T) {
	client := &MilvusClient{}

	//lint:ignore SA1012 This test verifies that nil contexts are rejected.
	require.EqualError(t, client.Close(nil), "milvus: nil context")
	require.NoError(t, client.Close(context.Background()))
}

func TestMilvusClientRejectsInvalidInputBeforeCallingSDK(t *testing.T) {
	client := &MilvusClient{}

	require.EqualError(
		t,
		//lint:ignore SA1012 This test verifies validation before context use.
		client.CreateCollection(nil, "collection", 0, ""),
		"collection dimension must be greater than zero",
	)
	require.EqualError(
		t,
		//lint:ignore SA1012 This test verifies validation before context use.
		client.CreateCollectionWithSchema(nil, nil, 1),
		"collection schema must not be nil",
	)
	//lint:ignore SA1012 This test verifies validation before context use.
	_, err := client.SearchVectors(nil, "collection", [][]float32{{1}}, 0, SearchOptions{})
	require.EqualError(t, err, "topK must be greater than zero")
}

func TestValidateCollectionRejectsVectorDimensionMismatch(t *testing.T) {
	expected := entity.NewSchema().
		WithName("vectors").
		WithField(entity.NewField().WithName("vector").WithDataType(entity.FieldTypeFloatVector).WithDim(128))
	actual := &entity.Collection{
		Name:     "vectors",
		ShardNum: 1,
		Schema: entity.NewSchema().
			WithName("vectors").
			WithField(entity.NewField().WithName("vector").WithDataType(entity.FieldTypeFloatVector).WithDim(256)),
	}

	require.ErrorContains(t, validateCollection(actual, expected, 1), `field "vector" does not match`)
}

func TestValidateCollectionPreservesTypeParameterNilness(t *testing.T) {
	expected := entity.NewSchema().
		WithName("vectors").
		WithField(&entity.Field{Name: "id", DataType: entity.FieldTypeInt64})
	actual := &entity.Collection{
		Name:     "vectors",
		ShardNum: 1,
		Schema: entity.NewSchema().
			WithName("vectors").
			WithField(&entity.Field{
				Name:       "id",
				DataType:   entity.FieldTypeInt64,
				TypeParams: map[string]string{},
			}),
	}

	require.ErrorContains(t, validateCollection(actual, expected, 1), `field "id" does not match`)
}

func TestBuildSearchOptionIncludesExplicitANNContract(t *testing.T) {
	annParam := milvusindex.NewHNSWAnnParam(64)
	option := buildSearchOption(
		"vectors",
		5,
		[]entity.Vector{entity.FloatVector{1, 2}},
		SearchOptions{
			ANNField:     "embedding",
			OutputFields: []string{"title"},
			MetricType:   entity.COSINE,
			ANNParam:     annParam,
			Filter:       "active == true",
		},
	)

	request, err := option.Request()
	require.NoError(t, err)
	require.Equal(t, "vectors", request.CollectionName)
	require.Equal(t, []string{"title"}, request.OutputFields)
	require.Equal(t, "active == true", request.Dsl)

	searchParams := entity.KvPairsMap(request.SearchParams)
	require.Equal(t, "COSINE", searchParams["metric_type"])
	var params map[string]any
	require.NoError(t, json.Unmarshal([]byte(searchParams["params"]), &params))
	require.Equal(t, float64(64), params["ef"])
}
