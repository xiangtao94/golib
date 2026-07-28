package milvus

import (
	"encoding/json"
	"testing"

	"github.com/milvus-io/milvus/client/v2/entity"
	milvusindex "github.com/milvus-io/milvus/client/v2/index"
	"github.com/stretchr/testify/require"
)

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

	idx, err := NewIndexByType(milvusindex.IvfFlat, entity.L2, params)

	require.NoError(t, err)
	require.Equal(t, map[string]string{"nlist": "1024"}, params)
	require.Equal(t, "1024", idx.Params()["nlist"])
	require.Equal(t, string(milvusindex.IvfFlat), idx.Params()[milvusindex.IndexTypeKey])
	require.Equal(t, string(entity.L2), idx.Params()[milvusindex.MetricTypeKey])
}

func TestMilvusClientRejectsInvalidInputBeforeCallingSDK(t *testing.T) {
	client := &MilvusClient{}

	require.EqualError(
		t,
		client.CreateCollection(nil, "collection", 0, ""),
		"collection dimension must be greater than zero",
	)
	require.EqualError(
		t,
		client.CreateCollectionWithSchema(nil, nil, 1),
		"collection schema must not be nil",
	)
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
