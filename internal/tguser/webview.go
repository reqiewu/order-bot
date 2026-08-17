package tguser

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
)

const webViewPlatform = "android"

// RequestInitData opens a Mini App and returns tgWebAppData.
// Empty shortName uses the bot's Main Mini App (t.me/BotUsername).
func (c *Client) RequestInitData(ctx context.Context, botUser, shortName string) (string, error) {
	api, err := c.API(ctx)
	if err != nil {
		return "", err
	}
	return RequestInitData(ctx, api, botUser, shortName)
}

// RequestInitData opens botUser's Mini App and extracts Telegram WebApp initData.
func RequestInitData(ctx context.Context, api *tg.Client, botUser, shortName string) (string, error) {
	if api == nil {
		return "", fmt.Errorf("tguser: nil api")
	}
	botUser = strings.TrimPrefix(strings.TrimSpace(botUser), "@")
	shortName = strings.TrimSpace(shortName)
	if botUser == "" {
		return "", fmt.Errorf("tguser: bot username required")
	}

	resolved, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: botUser})
	if err != nil {
		return "", fmt.Errorf("tguser: resolve @%s: %w", botUser, err)
	}
	bot, err := botFromResolved(resolved)
	if err != nil {
		return "", fmt.Errorf("tguser: @%s: %w", botUser, err)
	}

	if shortName == "" {
		return initFromMainWebView(ctx, api, botUser, bot)
	}

	res, err := api.MessagesRequestAppWebView(ctx, &tg.MessagesRequestAppWebViewRequest{
		Peer: bot.AsInputPeer(),
		App: &tg.InputBotAppShortName{
			BotID:     bot.AsInput(),
			ShortName: shortName,
		},
		Platform: webViewPlatform,
	})
	if err != nil {
		if tgerr.Is(err, "BOT_APP_SHORTNAME_INVALID") || tgerr.Is(err, "BOT_APP_INVALID") {
			return initFromMainWebView(ctx, api, botUser, bot)
		}
		return "", fmt.Errorf("tguser: requestAppWebView @%s/%s: %w", botUser, shortName, err)
	}
	return initFromURL(botUser+"/"+shortName, res.URL)
}

func initFromMainWebView(ctx context.Context, api *tg.Client, botUser string, bot *tg.User) (string, error) {
	res, err := api.MessagesRequestMainWebView(ctx, &tg.MessagesRequestMainWebViewRequest{
		Peer:     bot.AsInputPeer(),
		Bot:      bot.AsInput(),
		Platform: webViewPlatform,
	})
	if err != nil {
		return "", fmt.Errorf("tguser: requestMainWebView @%s: %w", botUser, err)
	}
	return initFromURL("@"+botUser, res.URL)
}

func initFromURL(label, raw string) (string, error) {
	initData, err := InitDataFromWebViewURL(raw)
	if err != nil {
		return "", fmt.Errorf("tguser: %s: %w", label, err)
	}
	return initData, nil
}

func botFromResolved(res *tg.ContactsResolvedPeer) (*tg.User, error) {
	if res == nil {
		return nil, fmt.Errorf("empty resolve result")
	}
	peer, ok := res.Peer.(*tg.PeerUser)
	if !ok {
		return nil, fmt.Errorf("resolved peer is not a user")
	}
	for _, u := range res.Users {
		user, ok := u.AsNotEmpty()
		if !ok {
			continue
		}
		if user.ID == peer.UserID {
			return user, nil
		}
	}
	return nil, fmt.Errorf("resolved user missing from payload")
}

// InitDataFromWebViewURL extracts and unescapes tgWebAppData from a Mini App URL
// (query or hash fragment).
func InitDataFromWebViewURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty webview url")
	}
	const key = "tgWebAppData="
	i := strings.Index(raw, key)
	if i < 0 {
		return "", fmt.Errorf("webview url has no tgWebAppData")
	}
	rest := raw[i+len(key):]
	if j := strings.Index(rest, "&tgWebAppVersion"); j >= 0 {
		rest = rest[:j]
	} else if j := strings.Index(rest, "&tgWebApp"); j >= 0 {
		rest = rest[:j]
	} else if j := strings.Index(rest, "&"); j >= 0 {
		rest = rest[:j]
	}
	decoded, err := url.QueryUnescape(rest)
	if err != nil {
		decoded, err = url.PathUnescape(rest)
		if err != nil {
			return "", fmt.Errorf("unescape tgWebAppData: %w", err)
		}
	}
	// Some Mini Apps (Getgems) nest percent-encoding once more.
	if strings.Contains(decoded, "%") {
		if twice, err2 := url.QueryUnescape(decoded); err2 == nil && twice != "" {
			decoded = twice
		}
	}
	decoded = strings.TrimSpace(decoded)
	if decoded == "" {
		return "", fmt.Errorf("empty tgWebAppData")
	}
	return decoded, nil
}
