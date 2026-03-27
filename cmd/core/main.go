package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/mintrage/linkguard/proto" // Импортируем наш сгенерированный код
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status" // Понадобится для тестирования
)

// linkServer - наша структура, которая реализует интерфейс из proto-файла
type linkServer struct {
	pb.UnimplementedLinkServiceServer // Обязательное встраивание для обратной совместимости
	rdb                               *redis.Client
}

var (
	linksCreated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "linkguard_created_total",
		Help: "Общее количество созданных коротких ссылок",
	})
	redirectsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "linkguard_redirects_total",
		Help: "Количество попыток перехода по ссылкам",
	}, []string{"status"})
)

// 1. Реализация метода CreateLink
func (s *linkServer) CreateLink(ctx context.Context, req *pb.CreateLinkRequest) (*pb.CreateLinkResponse, error) {
	originalURL := req.GetOriginalUrl()
	if originalURL == "" {
		return nil, status.Errorf(codes.InvalidArgument, "URL не может быть пустым")
	}
	fmt.Printf("Пришел запрос на сокращение: %s\n", originalURL)
	shortCode := generateShortCode(6)
	err := s.rdb.Set(ctx, shortCode, originalURL, 0).Err()
	if err != nil {
		log.Printf("Ошибка при сохранении в Redis: %v", err)
		return nil, status.Error(codes.Internal, "Не удалось сохранить ссылку")
	}

	fmt.Printf("✅ Создана ссылка: %s -> %s\n", shortCode, originalURL)
	linksCreated.Inc()

	return &pb.CreateLinkResponse{
		ShortLink: shortCode,
	}, nil
}

// 2. Реализация метода GetOriginalLink
func (s *linkServer) GetOriginalLink(ctx context.Context, req *pb.GetOriginalLinkRequest) (*pb.GetOriginalLinkResponse, error) {
	shortCode := req.GetShortLink()

	if shortCode == "" {
		return nil, status.Error(codes.InvalidArgument, "Короткий URL не может быть пустым!")
	}
	fmt.Printf("Запрошенный короткий код: %s\n", shortCode)
	originalURL, err := s.rdb.Get(ctx, shortCode).Result()
	if err == redis.Nil {
		redirectsTotal.WithLabelValues("miss").Inc()
		return nil, status.Error(codes.NotFound, "Ссылка не найдена")
	} else if err != nil {
		log.Printf("Ошибка чтения из Redis: %v", err)
		return nil, status.Error(codes.Internal, "Внутренняя ошибка базы данных")
	}

	fmt.Printf("🔍 Переход по ссылке: %s -> %s\n", shortCode, originalURL)
	redirectsTotal.WithLabelValues("hit").Inc()

	return &pb.GetOriginalLinkResponse{
		OriginalUrl: originalURL,
	}, nil
}

func generateShortCode(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[r.Intn(len(charset))]
	}
	return string(b)
}

func main() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       0,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("🚨 Ошибка подключения к Redis: %v. Проверь, запущен ли Docker контейнер!", err)
	}
	log.Println("✅ Успешно подключились к Redis!")

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("📊 Prometheus метрики доступны на http://localhost:2112/metrics")
		err := http.ListenAndServe(":2112", nil)
		if err != nil {
			log.Fatalf("⚠️ Ошибка запуска сервера метрик: %v", err)
		}

	}()

	// 1. Открываем TCP-порт (50051 - стандартный порт для gRPC)
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Не удалось прослушать порт: %v", err)
	}

	// 2. Создаем инстанс gRPC-сервера
	grpcServer := grpc.NewServer()

	// 3. Регистрируем нашу реализацию сервера в gRPC
	pb.RegisterLinkServiceServer(grpcServer, &linkServer{
		rdb: rdb,
	})

	// 4. Включаем рефлексию (чтобы мы могли тестировать сервер из консоли)
	reflection.Register(grpcServer)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("Ошибка сервера: %v", err)
		}
	}()
	log.Println("🚀 Core gRPC сервер запущен на порту 50051...")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Получен сигнал на остановку, начинаем Graceful Shutdown...")

	grpcServer.GracefulStop()
	rdb.Close()

	log.Println("✅ Core успешно остановлен.")

}
