package app

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/airsss993/histproject/backend/internal/platform/health"
)

type namedChecker string

func (c namedChecker) Name() string             { return string(c) }
func (namedChecker) Ping(context.Context) error { return nil }

func TestOpenAllClosesResourcesInReverseOrder(t *testing.T) {
	var closed []string
	resources, err := openAll(context.Background(), []dependencyOpener{
		fakeOpener("postgres", &closed, nil),
		fakeOpener("redis", &closed, nil),
		fakeOpener("minio", &closed, nil),
	})
	if err != nil {
		t.Fatalf("openAll() error = %v", err)
	}

	dependencies := &Dependencies{resources: resources}
	if err := dependencies.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if want := []string{"minio", "redis", "postgres"}; !reflect.DeepEqual(closed, want) {
		t.Fatalf("close order = %#v, want %#v", closed, want)
	}
}

func TestOpenAllRollsBackOpenedResources(t *testing.T) {
	var closed []string
	want := errors.New("redis unavailable")

	resources, err := openAll(context.Background(), []dependencyOpener{
		fakeOpener("postgres", &closed, nil),
		fakeOpener("redis", &closed, want),
		fakeOpener("minio", &closed, nil),
	})

	if !errors.Is(err, want) {
		t.Fatalf("openAll() error = %v, want Redis error", err)
	}
	if resources != nil {
		t.Fatal("openAll() resources are not nil")
	}
	if wantClosed := []string{"postgres"}; !reflect.DeepEqual(closed, wantClosed) {
		t.Fatalf("closed = %#v, want %#v", closed, wantClosed)
	}
}

func TestDependenciesCheckersReturnsAllResources(t *testing.T) {
	dependencies := &Dependencies{resources: []dependencyResource{
		{checker: namedChecker("postgres")},
		{checker: namedChecker("redis")},
		{checker: namedChecker("minio")},
	}}

	checkers := dependencies.Checkers()

	want := []string{"postgres", "redis", "minio"}
	got := make([]string, 0, len(checkers))
	for _, checker := range checkers {
		got = append(got, checker.Name())
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("checkers = %#v, want %#v", got, want)
	}
}

func fakeOpener(name string, closed *[]string, openErr error) dependencyOpener {
	return func(context.Context) (dependencyResource, error) {
		if openErr != nil {
			return dependencyResource{}, openErr
		}
		return dependencyResource{
			checker: namedChecker(name),
			close: func() error {
				*closed = append(*closed, name)
				return nil
			},
		}, nil
	}
}

var _ health.Checker = namedChecker("")
