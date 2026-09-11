package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
)

func Test_MetricsStorage(t *testing.T) {
	ctx := context.Background()

	newMock := func(ctrl *gomock.Controller) *MockStorage {
		m := NewMockStorage(ctrl)
		m.EXPECT().Close(gomock.Any()).Return(nil).AnyTimes()
		return m
	}

	t.Run("counts get hits and misses", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		inner := newMock(ctrl)
		inner.EXPECT().Get(gomock.Any(), "hit").Return(GetResponse{OutputID: []byte("o")}, true, nil).Times(1)
		inner.EXPECT().Get(gomock.Any(), "miss").Return(GetResponse{}, false, nil).Times(1)

		ms := NewMetricsStorage(inner, 0).(*metrics)
		ms.Get(ctx, "hit")
		ms.Get(ctx, "miss")

		assertEqual(t, int64(2), ms.GetCmd)
		assertEqual(t, int64(1), ms.GetMissCmd)
		assertEqual(t, int64(0), ms.Errors)

		ms.Close(ctx)
	})

	t.Run("counts put operations and sizes", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		inner := newMock(ctrl)
		inner.EXPECT().Put(gomock.Any(), gomock.Any()).Return("", nil).Times(3)

		ms := NewMetricsStorage(inner, 0).(*metrics)
		for _, size := range []int64{100, 200, 300} {
			ms.Put(ctx, PutRequest{Body: strings.NewReader(""), BodySize: size})
		}

		assertEqual(t, int64(3), ms.PutCmd)
		assertEqual(t, int64(600), ms.PutTotalSize)
		assertEqual(t, int64(100), ms.PutMinSize)
		assertEqual(t, int64(300), ms.PutMaxSize)

		ms.Close(ctx)
	})

	t.Run("counts errors from get", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		inner := newMock(ctrl)
		inner.EXPECT().Get(gomock.Any(), gomock.Any()).
			Return(GetResponse{}, false, fmt.Errorf("boom")).Times(2)

		ms := NewMetricsStorage(inner, 0).(*metrics)
		ms.Get(ctx, "k1")
		ms.Get(ctx, "k2")

		assertEqual(t, int64(2), ms.Errors)
		ms.Close(ctx)
	})

	t.Run("counts errors from put", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		inner := newMock(ctrl)
		inner.EXPECT().Put(gomock.Any(), gomock.Any()).
			Return("", fmt.Errorf("disk full")).Times(1)

		ms := NewMetricsStorage(inner, 0).(*metrics)
		ms.Put(ctx, PutRequest{Body: strings.NewReader("x"), BodySize: 1})

		assertEqual(t, int64(1), ms.Errors)
		ms.Close(ctx)
	})

	t.Run("computes avg time after close", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		inner := newMock(ctrl)
		inner.EXPECT().Get(gomock.Any(), gomock.Any()).Return(GetResponse{}, false, nil).Times(4)
		inner.EXPECT().Put(gomock.Any(), gomock.Any()).Return("", nil).Times(2)

		ms := NewMetricsStorage(inner, 0).(*metrics)
		for i := 0; i < 4; i++ {
			ms.Get(ctx, "k")
		}
		for i := 0; i < 2; i++ {
			ms.Put(ctx, PutRequest{Body: strings.NewReader(""), BodySize: 0})
		}
		ms.Close(ctx)

		// avg = sum / count, must be non-negative
		if ms.GetCmdAvgTime < 0 {
			t.Fatalf("GetCmdAvgTime should be non-negative, got %d", ms.GetCmdAvgTime)
		}
		if ms.PutCmdAvgTime < 0 {
			t.Fatalf("PutCmdAvgTime should be non-negative, got %d", ms.PutCmdAvgTime)
		}
	})

	t.Run("min/max time are updated correctly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		inner := newMock(ctrl)
		inner.EXPECT().Get(gomock.Any(), gomock.Any()).Return(GetResponse{}, false, nil).AnyTimes()

		ms := NewMetricsStorage(inner, 0).(*metrics)
		ms.Get(ctx, "k")
		ms.Get(ctx, "k")

		if ms.GetCmdMinTime > ms.GetCmdMaxTime {
			t.Fatalf("MinTime (%d) > MaxTime (%d)", ms.GetCmdMinTime, ms.GetCmdMaxTime)
		}
		ms.Close(ctx)
	})

	t.Run("printStat does not panic (level 1)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		inner := newMock(ctrl)
		inner.EXPECT().Get(gomock.Any(), gomock.Any()).Return(GetResponse{}, false, nil).Times(1)
		inner.EXPECT().Put(gomock.Any(), gomock.Any()).Return("", nil).Times(1)

		ms := NewMetricsStorage(inner, 1)
		ms.Get(ctx, "k")
		ms.Put(ctx, PutRequest{Body: strings.NewReader(""), BodySize: 0})
		assertNoErr(t, ms.Close(ctx))
	})

	t.Run("printAllStat does not panic (level 2)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		inner := newMock(ctrl)
		inner.EXPECT().Get(gomock.Any(), gomock.Any()).Return(GetResponse{}, false, nil).Times(1)
		inner.EXPECT().Put(gomock.Any(), gomock.Any()).Return("", nil).Times(1)

		ms := NewMetricsStorage(inner, 2)
		ms.Get(ctx, "k")
		ms.Put(ctx, PutRequest{Body: strings.NewReader(""), BodySize: 0})
		assertNoErr(t, ms.Close(ctx))
	})

	t.Run("printAllStat with zero operations does not panic (level 2)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		inner := newMock(ctrl)

		ms := NewMetricsStorage(inner, 2)
		assertNoErr(t, ms.Close(ctx))
	})

	t.Run("safeDiv returns -1 for zero divisor", func(t *testing.T) {
		if got := safeDiv(100, 0); got != -1 {
			t.Fatalf("safeDiv(100,0) = %d, want -1", got)
		}
	})

	t.Run("humanSize formats correctly", func(t *testing.T) {
		cases := []struct {
			bytes int64
			want  string
		}{
			{0, "0 B"},
			{1023, "1023 B"},
			{1024, "1.0 KB"},
			{1024 * 1024, "1.0 MB"},
			{1024 * 1024 * 1024, "1.0 GB"},
		}
		for _, c := range cases {
			got := humanSize(c.bytes)
			if got != c.want {
				t.Errorf("humanSize(%d) = %q, want %q", c.bytes, got, c.want)
			}
		}
	})
}
