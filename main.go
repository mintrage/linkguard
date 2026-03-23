package main

import (
	"context"
	"fmt"
	"log"
	"net"

	pb "github.com/mintrage/linkguard/proto" // Импортируем наш сгенерированный код
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection" // Понадобится для тестирования
)

// linkServer - наша структура, которая реализует интерфейс из proto-файла
type linkServer struct {
	pb.UnimplementedLinkServiceServer // Обязательное встраивание для обратной совместимости
}

// 1. Реализация метода CreateLink
func (s *linkServer) CreateLink(ctx context.Context, req *pb.CreateLinkRequest) (*pb.CreateLinkResponse, error) {
	// Senior tip: В gRPC лучше читать поля через встроенные геттеры (Get...),
	// так как они безопасно обрабатывают nil-значения.
	fmt.Printf("Пришел запрос на сокращение: %s\n", req.GetOriginalUrl())

	// Пока БД нет, делаем хардкод-заглушку (mock)
	fakeShortLink := "go.dev/xyz"

	return &pb.CreateLinkResponse{
		ShortLink: fakeShortLink,
	}, nil
}

// ------------------------------------------------------------------
// ТВОЯ ЗАДАЧА: Написать метод GetOriginalLink
// Подсказка: он должен принимать req *pb.GetOriginalLinkRequest
// и возвращать (*pb.GetOriginalLinkResponse, error)
// Внутри пока просто выведи в консоль запрошенный короткий код
// и верни захардкоженный длинный URL (например, "https://google.com").
// ------------------------------------------------------------------

func (s *linkServer) GetOriginalLink(ctx context.Context, req *pb.GetOriginalLinkRequest) (*pb.GetOriginalLinkResponse, error) {
	fmt.Printf("Запрошенный короткий код: %s\n", req.GetShortLink())

	fakeOriginalUrl := "https://google.com"

	return &pb.GetOriginalLinkResponse{
		OriginalUrl: fakeOriginalUrl,
	}, nil
}

func main() {
	// 1. Открываем TCP-порт (50051 - стандартный порт для gRPC)
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Не удалось прослушать порт: %v", err)
	}

	// 2. Создаем инстанс gRPC-сервера
	grpcServer := grpc.NewServer()

	// 3. Регистрируем нашу реализацию сервера в gRPC
	pb.RegisterLinkServiceServer(grpcServer, &linkServer{})

	// 4. Включаем рефлексию (чтобы мы могли тестировать сервер из консоли)
	reflection.Register(grpcServer)

	log.Println("🚀 Core gRPC сервер запущен на порту 50051...")

	// 5. Запускаем бесконечный цикл прослушивания
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Ошибка сервера: %v", err)
	}
}
