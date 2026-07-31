// Package steamapi is a small client for Steam's public Web API.
//
// It covers what the provider's read-only kinds need and nothing else. Two
// things are worth knowing about this API before reading further:
//
//   - It does not accept the local client's session. Being signed in to Steam
//     on this machine grants nothing here; every call needs a key issued at
//     steamcommunity.com/dev/apikey.
//   - It is read-only for everything a player owns. Valve publishes no endpoint
//     to buy a game, edit a wishlist, add a friend or change a profile; the
//     write APIs that exist are for publishers acting on their own titles. So
//     the kinds built on this package serve `get` and `describe` and say so
//     rather than pretending an `apply` could work.
package steamapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const baseURL = "https://api.steampowered.com"

// storeURL serves the wishlist, which lives on the store host rather than the
// API host and takes no key at all — it reads a public profile.
const storeURL = "https://api.steampowered.com"

// Client talks to the Web API on behalf of one account.
type Client struct {
	key     string
	steamID string
	http    *http.Client
}

// ErrNoCredentials is what every call returns when the key or the id is
// missing. It is separate so the handlers can explain the setup once, in terms
// of what the user has to go and get.
type ErrNoCredentials struct{ Missing string }

func (e *ErrNoCredentials) Error() string {
	return fmt.Sprintf("the Steam Web API needs %s: being signed in to the Steam client is not enough, since the client's session does not authenticate this API.\n"+
		"Get a key at https://steamcommunity.com/dev/apikey and set STEAM_API_KEY; the account is taken from the local login, or set STEAM_ID to override it", e.Missing)
}

// New builds a client. It reports missing credentials as an error rather than
// failing later, so `get` explains the setup instead of returning nothing.
func New(key, steamID string) (*Client, error) {
	switch {
	case key == "" && steamID == "":
		return nil, &ErrNoCredentials{Missing: "an API key and a SteamID"}
	case key == "":
		return nil, &ErrNoCredentials{Missing: "an API key"}
	case steamID == "":
		return nil, &ErrNoCredentials{Missing: "a SteamID"}
	}
	return &Client{
		key:     key,
		steamID: steamID,
		// Steam's API is occasionally slow and occasionally hangs; a bounded
		// timeout keeps `whoctl get` from doing the same.
		http: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// SteamID returns the account every call is made for.
func (c *Client) SteamID() string { return c.steamID }

// get calls an endpoint and decodes the JSON into out.
func (c *Client) get(ctx context.Context, host, path string, params url.Values, out any) error {
	if params == nil {
		params = url.Values{}
	}
	if c.key != "" {
		params.Set("key", c.key)
	}
	endpoint := host + path + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling the Steam Web API: %w", err)
	}
	defer resp.Body.Close()

	// The key never reaches an error message: it is a credential, and a failed
	// command is exactly the output most likely to be pasted into a bug report.
	if resp.StatusCode != http.StatusOK {
		return apiError(resp.StatusCode, path)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("reading the Steam Web API response: %w", err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decoding the Steam Web API response from %s: %w", path, err)
	}
	return nil
}

func apiError(status int, path string) error {
	endpoint := strings.TrimPrefix(path, "/")
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("the Steam Web API refused the request to %s: the key may be wrong, or the profile may be private — game details need the profile's game details set to public", endpoint)
	case http.StatusTooManyRequests:
		return fmt.Errorf("the Steam Web API is rate limiting: it allows 100,000 calls a day per key, try again shortly")
	default:
		return fmt.Errorf("the Steam Web API returned %d for %s", status, endpoint)
	}
}

// Game is an owned or recently played title.
type Game struct {
	AppID                    int64  `json:"appid"`
	Name                     string `json:"name"`
	PlaytimeForever          int64  `json:"playtime_forever"`
	Playtime2Weeks           int64  `json:"playtime_2weeks"`
	PlaytimeWindows          int64  `json:"playtime_windows_forever"`
	PlaytimeMac              int64  `json:"playtime_mac_forever"`
	PlaytimeLinux            int64  `json:"playtime_linux_forever"`
	PlaytimeDeck             int64  `json:"playtime_deck_forever"`
	RTimeLastPlayed          int64  `json:"rtime_last_played"`
	ImgIconURL               string `json:"img_icon_url"`
	HasCommunityVisibleStats bool   `json:"has_community_visible_stats"`
}

// OwnedGames lists everything the account owns.
func (c *Client) OwnedGames(ctx context.Context) ([]Game, error) {
	var resp struct {
		Response struct {
			GameCount int    `json:"game_count"`
			Games     []Game `json:"games"`
		} `json:"response"`
	}
	params := url.Values{
		"steamid":                   {c.steamID},
		"include_appinfo":           {"1"},
		"include_played_free_games": {"1"},
	}
	if err := c.get(ctx, baseURL, "/IPlayerService/GetOwnedGames/v1/", params, &resp); err != nil {
		return nil, err
	}
	// An empty response with no error is what a private profile returns, which
	// is a different problem from an empty library.
	if resp.Response.Games == nil && resp.Response.GameCount == 0 {
		return nil, fmt.Errorf("the Steam Web API returned no games: the profile's game details are probably private (Profile → Privacy Settings → Game details)")
	}
	return resp.Response.Games, nil
}

// RecentlyPlayed lists what the account played in the last two weeks.
func (c *Client) RecentlyPlayed(ctx context.Context) ([]Game, error) {
	var resp struct {
		Response struct {
			Games []Game `json:"games"`
		} `json:"response"`
	}
	params := url.Values{"steamid": {c.steamID}}
	err := c.get(ctx, baseURL, "/IPlayerService/GetRecentlyPlayedGames/v1/", params, &resp)
	return resp.Response.Games, err
}

// WishlistItem is one entry of the account's wishlist.
type WishlistItem struct {
	AppID     int64 `json:"appid"`
	Priority  int   `json:"priority"`
	DateAdded int64 `json:"date_added"`
	Name      string
}

// Wishlist returns the account's wishlist, newest first by priority.
//
// The wishlist is read-only here for the same reason as everything else: Valve
// exposes no endpoint to add or remove an entry, only the store's own web
// session does that.
func (c *Client) Wishlist(ctx context.Context) ([]WishlistItem, error) {
	var resp struct {
		Response struct {
			Items []WishlistItem `json:"items"`
		} `json:"response"`
	}
	params := url.Values{"steamid": {c.steamID}}
	if err := c.get(ctx, storeURL, "/IWishlistService/GetWishlist/v1/", params, &resp); err != nil {
		return nil, err
	}
	return resp.Response.Items, nil
}

// Friend is one entry of the account's friend list.
type Friend struct {
	SteamID      string `json:"steamid"`
	Relationship string `json:"relationship"`
	FriendSince  int64  `json:"friend_since"`
}

// Friends returns the account's friend list. The list carries only ids, so the
// caller pairs it with PlayerSummaries to get names.
func (c *Client) Friends(ctx context.Context) ([]Friend, error) {
	var resp struct {
		FriendsList struct {
			Friends []Friend `json:"friends"`
		} `json:"friendslist"`
	}
	params := url.Values{"steamid": {c.steamID}, "relationship": {"friend"}}
	if err := c.get(ctx, baseURL, "/ISteamUser/GetFriendList/v1/", params, &resp); err != nil {
		return nil, err
	}
	return resp.FriendsList.Friends, nil
}

// Player is a profile summary.
type Player struct {
	SteamID                  string `json:"steamid"`
	PersonaName              string `json:"personaname"`
	ProfileURL               string `json:"profileurl"`
	Avatar                   string `json:"avatarfull"`
	PersonaState             int    `json:"personastate"`
	CommunityVisibilityState int    `json:"communityvisibilitystate"`
	RealName                 string `json:"realname"`
	LocCountryCode           string `json:"loccountrycode"`
	TimeCreated              int64  `json:"timecreated"`
	LastLogoff               int64  `json:"lastlogoff"`
	GameID                   string `json:"gameid"`
	GameExtraInfo            string `json:"gameextrainfo"`
}

// PlayerSummaries looks up profiles by id. Steam accepts up to 100 per call, so
// longer lists are split.
func (c *Client) PlayerSummaries(ctx context.Context, ids []string) ([]Player, error) {
	var out []Player
	const batch = 100
	for start := 0; start < len(ids); start += batch {
		end := min(start+batch, len(ids))
		var resp struct {
			Response struct {
				Players []Player `json:"players"`
			} `json:"response"`
		}
		params := url.Values{"steamids": {strings.Join(ids[start:end], ",")}}
		if err := c.get(ctx, baseURL, "/ISteamUser/GetPlayerSummaries/v2/", params, &resp); err != nil {
			return nil, err
		}
		out = append(out, resp.Response.Players...)
	}
	return out, nil
}

// Achievement is one achievement of one game.
type Achievement struct {
	APIName     string `json:"apiname"`
	Achieved    int    `json:"achieved"`
	UnlockTime  int64  `json:"unlocktime"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Achievements returns the account's achievements for one app.
func (c *Client) Achievements(ctx context.Context, appID string) ([]Achievement, error) {
	var resp struct {
		PlayerStats struct {
			Success      bool          `json:"success"`
			Error        string        `json:"error"`
			Achievements []Achievement `json:"achievements"`
		} `json:"playerstats"`
	}
	params := url.Values{"steamid": {c.steamID}, "appid": {appID}, "l": {"english"}}
	if err := c.get(ctx, baseURL, "/ISteamUserStats/GetPlayerAchievements/v1/", params, &resp); err != nil {
		return nil, err
	}
	if !resp.PlayerStats.Success {
		// "Requested app has no stats" is the normal answer for a game without
		// achievements, and reads better than an empty list with no reason.
		reason := resp.PlayerStats.Error
		if reason == "" {
			reason = "the app has no achievements, or the profile is private"
		}
		return nil, fmt.Errorf("no achievements for app %s: %s", appID, reason)
	}
	return resp.PlayerStats.Achievements, nil
}

// SteamLevel returns the account's community level.
func (c *Client) SteamLevel(ctx context.Context) (int, error) {
	var resp struct {
		Response struct {
			PlayerLevel int `json:"player_level"`
		} `json:"response"`
	}
	params := url.Values{"steamid": {c.steamID}}
	err := c.get(ctx, baseURL, "/IPlayerService/GetSteamLevel/v1/", params, &resp)
	return resp.Response.PlayerLevel, err
}
