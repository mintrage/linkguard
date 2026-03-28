package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"time"

	pb "github.com/mintrage/linkguard/proto"
	tele "gopkg.in/telebot.v3"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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
		matched, _ := regexp.MatchString(`^(https?://)?[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}(/.*)?$`, userText)
		if !matched {
			return c.Send("🤔 Это не похоже на правильную ссылку. Пожалуйста, отправь валидный URL (например, google.com)")
		}

		userID := strconv.FormatInt(c.Sender().ID, 10)
		clientID := "tg_user:" + userID
		ctx := metadata.AppendToOutgoingContext(context.Background(), "client_id", clientID)

		req := &pb.CreateLinkRequest{
			OriginalUrl: userText,
		}

		resp, err := grpcClient.CreateLink(ctx, req)

		if err != nil {
			if st, ok := status.FromError(err); ok {
				if st.Code() == codes.ResourceExhausted {
					return c.Send("⏳ Воу, полегче! Лимит запросов исчерпан. Подожди минутку.")
				}
				if st.Code() == codes.InvalidArgument {
					return c.Send("⚠️ Ядро ругается: неверный формат ссылки.")
				}
			}
			log.Printf("Неизвестная ошибка: %v", err)
			return c.Send("❌ Упс, сервер сейчас недоступен. Попробуй позже.")
		}

		responseData := resp.GetShortLink()

		return c.Send(fmt.Sprintf("Твоя ссылка: http://localhost:8080/%s", responseData))
	})

	log.Println("🤖 Telegram-бот запущен!")
	b.Start() // Эта функция заблокирует поток, как и HTTP/gRPC серверы
}
