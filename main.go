package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	"github.com/wader/goutubedl"
	"golang.org/x/exp/slices"
)

var dlQueue DownloadQueue

var (
	telegramUploader *uploader.Uploader
	telegramSender   *message.Sender
)

func handleCmdDLP(ctx context.Context, entities tg.Entities, u *tg.UpdateNewMessage, msg *tg.Message) {
	format := "video"
	s := strings.Split(msg.Message, " ")
	if len(s) >= 2 && s[0] == "mp3" {
		msg.Message = strings.Join(s[1:], " ")
		format = "mp3"
	}

	// Check if message is an URL.
	validURI := true
	uri, err := url.ParseRequestURI(msg.Message)
	if err != nil || (uri.Scheme != "http" && uri.Scheme != "https") {
		validURI = false
	} else {
		_, err = net.LookupHost(uri.Host)
		if err != nil {
			validURI = false
		}
	}
	if !validURI {
		fmt.Println("  (not an url)")
		_, _ = telegramSender.Reply(entities, u).Text(ctx, errorStr+": please enter an URL to download")
		return
	}

	dlQueue.Add(ctx, entities, u, msg.Message, format)
}

func handleCmdDLPCancel(ctx context.Context, entities tg.Entities, u *tg.UpdateNewMessage, msg *tg.Message) {
	dlQueue.CancelCurrentEntry(ctx, entities, u, msg.Message)
}

func handleMsg(ctx context.Context, entities tg.Entities, u *tg.UpdateNewMessage) error {
	msg, ok := u.Message.(*tg.Message)
	if !ok || msg.Out {
		// Outgoing message, not interesting.
		return nil
	}

	fromUser, fromGroup := resolveMsgSrc(msg)
	fromUsername := getFromUsername(entities, fromUser.UserID)

	fmt.Print("got message")
	if fromUsername != "" {
		fmt.Print(" from ", fromUsername, "#", fromUser.UserID)
	}
	fmt.Println(":", msg.Message)

	if fromGroup != nil {
		fmt.Print("  msg from group #", -fromGroup.ChatID)
		if !slices.Contains(params.AllowedGroupIDs, -fromGroup.ChatID) {
			fmt.Println(", group not allowed, ignoring")
			return nil
		}
		fmt.Println()
	} else {
		if !slices.Contains(params.AllowedUserIDs, fromUser.UserID) {
			fmt.Println("  user not allowed, ignoring")
			return nil
		}
	}

	// Check if message is a command.
	if msg.Message[0] == '/' || msg.Message[0] == '!' {
		cmd := strings.Split(msg.Message, " ")[0]
		msg.Message = strings.TrimPrefix(msg.Message, cmd+" ")
		if strings.Contains(cmd, "@") {
			cmd = strings.Split(cmd, "@")[0]
		}
		cmd = cmd[1:] // Cutting the command character.
		switch cmd {
		case "dlp":
			handleCmdDLP(ctx, entities, u, msg)
			return nil
		case "dlpcancel":
			handleCmdDLPCancel(ctx, entities, u, msg)
			return nil
		case "start":
			fmt.Println("  (start cmd)")
			if fromGroup == nil {
				_, _ = telegramSender.Reply(entities, u).Text(ctx, "🤖 Welcome! This bot downloads videos from various "+
					"supported sources and then re-uploads them to Telegram, so they can be viewed with Telegram's built-in "+
					"video player.")
			}
			return nil
		default:
			fmt.Println("  (invalid cmd)")
			if fromGroup == nil {
				_, _ = telegramSender.Reply(entities, u).Text(ctx, errorStr+": invalid command")
			}
			return nil
		}
	}

	if fromGroup == nil {
		handleCmdDLP(ctx, entities, u, msg)
	}
	return nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("yt-dlp-telegram-bot starting...")

	if err := params.Init(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	// Dispatcher handles incoming updates.
	dispatcher := tg.NewUpdateDispatcher()
	opts := telegram.Options{
		UpdateHandler: dispatcher,
	}
	var err error
	opts, err = telegram.OptionsFromEnvironment(opts)
	if err != nil {
		panic(fmt.Sprint("options from env err: ", err))
	}

	ticker := time.NewTicker(24 * time.Hour) // TODO: move to config
	go func() {
		defer ticker.Stop()

		for {
			select {
			case t := <-ticker.C:
				fmt.Printf("run yt-dlp version checker at %s", t.Format(time.RFC3339))
				s, updateNeeded, gotError := ytdlpVersionCheckGetStr(ctx)
				if gotError {
					sendTextToAdmins(ctx, s)
				} else if updateNeeded {
					goutubedl.Path, err = ytdlpDownloadLatest(ctx)
					if err != nil {
						panic(fmt.Sprint("error: ", err))
					}
					ytdlpVersionCheckStr, _, _ := ytdlpVersionCheckGetStr(ctx)
					sendTextToAdmins(ctx, "🤖 Bot updated, "+ytdlpVersionCheckStr)
				}
			case <-ctx.Done():
				fmt.Printf("stop yt-dlp version checker: %s\n", ctx.Err())
				return
			}
		}
	}()

	client := telegram.NewClient(params.ApiID, params.ApiHash, opts)
	err = client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			panic(fmt.Sprint("auth status err: ", err))
		}

		if !status.Authorized { // Not logged in?
			fmt.Println("logging in...")
			if _, err := client.Auth().Bot(ctx, params.BotToken); err != nil {
				panic(fmt.Sprint("login err: ", err))
			}
		}

		api := client.API()

		telegramUploader = uploader.NewUploader(api).WithProgress(dlUploader)
		telegramSender = message.NewSender(api).WithUploader(telegramUploader)

		goutubedl.Path, err = exec.LookPath(goutubedl.Path)
		if err != nil {
			goutubedl.Path, err = ytdlpDownloadLatest(ctx)
			if err != nil {
				panic(fmt.Sprint("error: ", err))
			}
		}

		dlQueue.Init(ctx)

		dispatcher.OnNewMessage(handleMsg)

		fmt.Println("telegram connection up")

		ytdlpVersionCheckStr, updateNeeded, _ := ytdlpVersionCheckGetStr(ctx)
		if updateNeeded {
			goutubedl.Path, err = ytdlpDownloadLatest(ctx)
			if err != nil {
				panic(fmt.Sprint("error: ", err))
			}
			ytdlpVersionCheckStr, _, _ = ytdlpVersionCheckGetStr(ctx)
		}
		fmt.Printf("%s %s\n", ytdlpVersionCheckStr, goutubedl.Path)

		sendTextToAdmins(ctx, "🤖 Bot started, "+ytdlpVersionCheckStr)

		<-ctx.Done()
		return nil
	})
	if err != nil {
		fmt.Printf("telegram client error: %s\n", err)
	}
}
