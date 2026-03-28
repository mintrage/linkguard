package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	pb "github.com/mintrage/linkguard/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type CreateRequest struct {
	URL string `json:"url"`
}

func main() {
	// redisAddr := os.Getenv("REDIS_ADDR")
	// if redisAddr == "" {
	// 	redisAddr = "localhost:6379"
	// }
	// rdb := redis.NewClient(&redis.Options{
	// 	Addr:     redisAddr,
	// 	Password: "",
	// 	DB:       0,
	// })
	// if err := rdb.Ping(context.Background()).Err(); err != nil {
	// 	log.Fatalf("🚨 Ошибка подключения к Redis: %v. Проверь, запущен ли Docker контейнер!", err)
	// 	return
	// }
	// log.Println("✅ Успешно подключились к Redis!")

	coreAddr := os.Getenv("CORE_ADDR")
	if coreAddr == "" {
		coreAddr = "localhost:50051"
	}
	conn, err := grpc.NewClient(coreAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Не удалось подключиться к gRPC серверу: %v", err)
	}
	defer conn.Close()

	grpcClient := pb.NewLinkServiceClient(conn)

	http.HandleFunc("/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Только POST запросы", http.StatusMethodNotAllowed)
			return
		}

		userIP := strings.Split(r.RemoteAddr, ":")[0]
		// key := "rate_limit:" + userIP
		// count, err := rdb.Incr(r.Context(), key).Result()
		// if err != nil {
		// 	log.Printf("Ошибка Rate Limiter: %v", err)
		// }
		// if count == 1 {
		// 	rdb.Expire(r.Context(), key, time.Minute).Err()
		// } else if count > 5 {
		// 	http.Error(w, "Слишком много запросов", http.StatusTooManyRequests)
		// 	return
		// }
		clientID := "gateway_ip:" + userIP
		ctx := metadata.AppendToOutgoingContext(r.Context(), "client_id", clientID)

		var reqData CreateRequest

		if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
			http.Error(w, "Неверный формат JSON", http.StatusBadRequest)
			return
		}

		defer r.Body.Close()

		req := &pb.CreateLinkRequest{
			OriginalUrl: reqData.URL,
		}

		resp, err := grpcClient.CreateLink(ctx, req)

		if err != nil {
			http.Error(w, "Ошибка создания ссылки", http.StatusInternalServerError)
			return
		}

		responseData := map[string]string{
			"short_link": resp.GetShortLink(),
		}

		w.Header().Set("Content-Type", "application/json")

		w.WriteHeader(http.StatusCreated)

		if err := json.NewEncoder(w).Encode(responseData); err != nil {
			http.Error(w, "Ошибка при формировании ответа", http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Только GET запросы", http.StatusMethodNotAllowed)
			return
		}

		userIP := strings.Split(r.RemoteAddr, ":")[0]
		clientID := userIP
		ctx := metadata.AppendToOutgoingContext(r.Context(), "client_id", clientID)

		shortCode := strings.TrimPrefix(r.URL.Path, "/")
		if shortCode == "" {
			http.Error(w, "Код не указан", http.StatusBadRequest)
			return
		}

		req := &pb.GetOriginalLinkRequest{
			ShortLink: shortCode,
		}

		resp, err := grpcClient.GetOriginalLink(ctx, req)

		if err != nil {
			http.Error(w, "Ссылка не найдена", http.StatusNotFound)
			return
		}

		http.Redirect(w, r, resp.GetOriginalUrl(), http.StatusFound)

	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: nil,
	}

	go func() {
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Printf("Ошибка запуска HTTP сервера: %v", err)
		}
	}()
	log.Println("🌐 API Gateway запущен на порту 8080...")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Получен сигнал на остановку, начинаем Graceful Shutdown...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Ошибка при мягком выключении: %v", err)
	}
	log.Println("✅ Gateway успешно остановлен.")

}
