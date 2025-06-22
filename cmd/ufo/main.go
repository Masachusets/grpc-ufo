package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	ufo_v1 "github.com/Masachusets/grpc-ufo/pkg/proto/ufo/v1"
)

const grpcPort = 50051

// ufoService реализует gRPC сервис для работы с наблюдениями НЛО
type ufoService struct {
	ufo_v1.UnimplementedUFOServiceServer

	mu        sync.RWMutex
	sightings map[string]*ufo_v1.Sighting
}

// Create создает новое наблюдение НЛО
func (s *ufoService) Create(_ context.Context, req *ufo_v1.CreateRequest) (*ufo_v1.CreateResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Генерируем UUID для нового наблюдения
	newUUID := uuid.NewString()

	sighting := &ufo_v1.Sighting{
		Uuid:      newUUID,
		Info:      req.GetInfo(),
		CreatedAt: timestamppb.New(time.Now()),
	}

	s.sightings[newUUID] = sighting

	log.Printf("Создано наблюдение с UUID %s", newUUID)

	return &ufo_v1.CreateResponse{Uuid: newUUID}, nil
}

// Get возвращает наблюдение НЛО по UUID
func (s *ufoService) Get(_ context.Context, req *ufo_v1.GetRequest) (*ufo_v1.GetResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sighting, ok := s.sightings[req.GetUuid()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "sigtsing with UUID %s not found", req.GetUuid())
	}

	return &ufo_v1.GetResponse{Sighting: sighting}, nil
}

// Get возвращает все наблюдения НЛО
func (s *ufoService) GetAll(_ context.Context, req *ufo_v1.GetAllRequest) (*ufo_v1.GetAllResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sightings := make([]*ufo_v1.Sighting, 0, len(s.sightings))

	for _, sighting := range s.sightings {
		sightings = append(sightings, sighting)
	}

	return &ufo_v1.GetAllResponse{Sighting: sightings}, nil
}

// Update обновляет существующее наблюдение НЛО
func (s *ufoService) Update(_ context.Context, req *ufo_v1.UpdateRequest) (*emptypb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sighting, ok := s.sightings[req.GetUuid()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "sigtsing with UUID %s not found", req.GetUuid())
	}

	if req.UpdateInfo == nil {
		return nil, status.Error(codes.InvalidArgument, "update info can not be nil")
	}

	// Обновляем поля, только если они были установлены в запросе
	if req.GetUpdateInfo().ObservedAt != nil {
		sighting.Info.ObservedAt = req.GetUpdateInfo().ObservedAt
	}

	if req.GetUpdateInfo().Location != nil {
		sighting.Info.Location = req.GetUpdateInfo().Location.Value
	}

	if req.GetUpdateInfo().Description != nil {
		sighting.Info.Description = req.GetUpdateInfo().Description.Value
	}

	if req.GetUpdateInfo().Color != nil {
		sighting.Info.Color = req.GetUpdateInfo().Color
	}

	if req.GetUpdateInfo().Sound != nil {
		sighting.Info.Sound = req.GetUpdateInfo().Sound
	}

	if req.GetUpdateInfo().DurationSeconds != nil {
		sighting.Info.DurationSeconds = req.GetUpdateInfo().DurationSeconds
	}

	sighting.UpdatedAt = timestamppb.New(time.Now())

	return &emptypb.Empty{}, nil
}

// Delete удаляет наблюдение НЛО (мягкое удаление - устанавливает deleted_at)
func (s *ufoService) Delete(_ context.Context, req *ufo_v1.DeleteRequest) (*emptypb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sighting, ok := s.sightings[req.GetUuid()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "sighting with UUID %s not found", req.GetUuid())
	}

	// Мягкое удаление - устанавливаем deleted_at
	sighting.DeletedAt = timestamppb.New(time.Now())

	return &emptypb.Empty{}, nil
}

func main() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Создаем gRPC сервер
	s := grpc.NewServer()

	// Регистрируем наш сервис
	service := &ufoService{
		sightings: make(map[string]*ufo_v1.Sighting),
	}

	ufo_v1.RegisterUFOServiceServer(s, service)

	// Включаем рефлексию для удобства тестирования
	reflection.Register(s)

	go func() {
		log.Printf("🚀 gRPC server listening on %s", lis.Addr())
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down gRPC server...")
	s.GracefulStop()

	log.Println("✅ Server stopped")
}
