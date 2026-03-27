package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	pb "github.com/mintrage/linkguard/proto"
	tele "gopkg.in/telebot.v3"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	tgToken := os.Getenv("TG_TOKEN")
	if tgToken == "" {
		log.Fatalf("Токен не указан")
	}
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

	pref := tele.Settings{
		Token:  tgToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatalf("Ошибка запуска бота: %v", err)
		return
	}
	b.Handle("/start", func(c tele.Context) error {
		welcomeText := "Привет! Я бот для сокращения ссылок.\nПросто отправь мне любой URL в таком формате: https://google.com"

		return c.Send(
			welcomeText,
			&tele.SendOptions{
				DisableWebPagePreview: true,
			},
		)
	})

	b.Handle(tele.OnText, func(c tele.Context) error {
		userText := c.Text()

		req := &pb.CreateLinkRequest{
			OriginalUrl: userText,
		}

		resp, err := grpcClient.CreateLink(context.Background(), req)

		if err != nil {
			return c.Send("Ошибка при сокращении")
		}

		responseData := resp.GetShortLink()

		return c.Send(fmt.Sprintf("Твоя ссылка: http://localhost:8080/%s", responseData))
	})

	log.Println("🤖 Telegram-бот запущен!")
	b.Start() // Эта функция заблокирует поток, как и HTTP/gRPC серверы
}
