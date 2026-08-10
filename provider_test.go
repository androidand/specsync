package specsync

import (
	"context"
	"testing"
)

func TestFindWithRetry_ImmediateHit(t *testing.T) {
	calls := 0
	ref, err := findWithRetry(context.Background(), func(ctx context.Context) (*Ref, error) {
		calls++
		return &Ref{ID: "1"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if ref == nil || ref.ID != "1" {
		t.Errorf("got %+v", ref)
	}
	if calls != 1 {
		t.Errorf("an immediate hit must not retry, got %d calls", calls)
	}
}

func TestFindWithRetry_ErrorNotRetried(t *testing.T) {
	calls := 0
	wantErr := context.Canceled
	_, err := findWithRetry(context.Background(), func(ctx context.Context) (*Ref, error) {
		calls++
		return nil, wantErr
	})
	if err != wantErr {
		t.Errorf("got %v", err)
	}
	if calls != 1 {
		t.Errorf("a real error must not retry, got %d calls", calls)
	}
}

func TestFindWithRetry_HitOnRetry(t *testing.T) {
	calls := 0
	ref, err := findWithRetry(context.Background(), func(ctx context.Context) (*Ref, error) {
		calls++
		if calls < 2 {
			return nil, nil // "not found yet" — simulates search-index propagation lag
		}
		return &Ref{ID: "2"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if ref == nil || ref.ID != "2" {
		t.Errorf("got %+v", ref)
	}
	if calls != 2 {
		t.Errorf("expected exactly one retry before the hit, got %d calls", calls)
	}
}

func TestFindWithRetry_ExhaustsAndReturnsNotFound(t *testing.T) {
	calls := 0
	ref, err := findWithRetry(context.Background(), func(ctx context.Context) (*Ref, error) {
		calls++
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if ref != nil {
		t.Errorf("got %+v, want nil after genuinely not found", ref)
	}
	if calls != len(findRetryDelays)+1 {
		t.Errorf("expected %d attempts (1 + retries), got %d", len(findRetryDelays)+1, calls)
	}
}

func TestFindWithRetry_ContextCancelledDuringWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, err := findWithRetry(ctx, func(ctx context.Context) (*Ref, error) {
		calls++
		if calls == 1 {
			cancel() // cancel during the backoff wait after the first miss
		}
		return nil, nil
	})
	if err != context.Canceled {
		t.Errorf("got %v, want context.Canceled", err)
	}
}
