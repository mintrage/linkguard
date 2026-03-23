package main

import (
	"encoding/json"
	"log"
	"net/http"

	pb "github.com/mintrage/linkguard/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type CreateRequest struct {
	URL string `json:"url"`
}

func main() {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
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

		var reqData CreateRequest

		if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
			http.Error(w, "Неверный формат JSON", http.StatusBadRequest)
			return
		}

		defer r.Body.Close()

		req := &pb.CreateLinkRequest{
			OriginalUrl: reqData.URL,
		}

		resp, err := grpcClient.CreateLink(r.Context(), req)

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

	log.Println("🌐 API Gateway запущен на порту 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Ошибка запуска HTTP сервера: %v", err)
	}
}
