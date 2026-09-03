package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
)

func Test(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockStorage := NewMockStorage(ctrl)
	mockStorage.EXPECT().Get(gomock.Any(), gomock.Any()).Return(GetResponse{}, false, nil).AnyTimes()
	mockStorage.EXPECT().Put(gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()
	mockStorage.EXPECT().Close(gomock.Any()).Return(nil).AnyTimes()
	// Wrap it with metrics
	metricsStorage := NewMetricsStorage(mockStorage)

	// Simulate some operations
	ctx := context.Background()

	// Perform some GET operations
	for i := 0; i < 5; i++ {
		metricsStorage.Get(ctx, fmt.Sprintf("key%d", i))
		time.Sleep(10 * time.Millisecond)
	}

	// Perform some PUT operations
	for i := 0; i < 3; i++ {
		req := PutRequest{
			Key:      fmt.Sprintf("key%d", i),
			BodySize: int64(100 * (i + 1)),
		}
		metricsStorage.Put(ctx, req)
		time.Sleep(15 * time.Millisecond)
	}

	// Close to trigger metrics printing
	metricsStorage.Close(ctx)
	//// MockStorage implements the Storage interface for testing
	//type MockStorage struct{}
	//func (m *MockStorage) Get(ctx context.Context, key string) (GetResponse, bool, error) {
	//	// Simulate some processing time
	//	time.Sleep(5 * time.Millisecond)
	//	return GetResponse{}, true, nil
	//}
	//func (m *MockStorage) Put(ctx context.Context, request PutRequest) (string, error) {
	//	// Simulate some processing time
	//	time.Sleep(10 * time.Millisecond)
	//	return "", nil
	//}
	//func (m *MockStorage) Close(ctx context.Context) error {
	//	return nil
}
