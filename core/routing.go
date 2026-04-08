package core

import (
	"regexp"
	"strings"
)

// geositeHints maps a few popular geosite categories to a small list of
// well-known domain suffixes. We don't parse the binary geosite.dat file —
// these hints are enough to give the user a reasonable preview of which
// outbound a service will go through.
var geositeHints = map[string][]string{
	"ru": {
		"vk.com", "vk.ru", "yandex.ru", "yandex.com", "mail.ru", "ok.ru",
		"gosuslugi.ru", "sber.ru", "sberbank.ru", "avito.ru",
		"wildberries.ru", "ozon.ru", "kinopoisk.ru", "rutube.ru",
		"dzen.ru", "rambler.ru", "lenta.ru", "rbc.ru",
	},
	// Anything blocked in Russia. Union of google/youtube/meta/twitter/discord/etc.
	"ru-blocked": {
		"google.com", "googleapis.com", "googleusercontent.com", "gstatic.com",
		"youtube.com", "youtu.be", "ytimg.com", "googlevideo.com",
		"facebook.com", "fb.com", "fbcdn.net",
		"instagram.com", "cdninstagram.com",
		"twitter.com", "x.com", "t.co", "twimg.com",
		"discord.com", "discord.gg", "discordapp.com", "discordapp.net", "discordcdn.com",
		"telegram.org", "t.me", "telegra.ph",
		"tiktok.com", "tiktokcdn.com", "byteoversea.com",
		"spotify.com", "scdn.co", "spotifycdn.com",
		"reddit.com", "redd.it", "redditmedia.com", "redditstatic.com",
		"linkedin.com", "licdn.com",
		"openai.com", "chatgpt.com", "oaistatic.com",
		"signal.org",
		"protonmail.com", "proton.me",
		"medium.com",
		"twitch.tv",
		"mega.nz",
	},
	"anthropic": {
		"anthropic.com", "claude.ai",
	},
	"google": {
		"google.com", "google.ru", "googleapis.com", "googleusercontent.com",
		"gstatic.com", "youtube.com", "youtu.be", "ytimg.com",
		"blogspot.com", "android.com", "chrome.com",
	},
	"youtube": {
		"youtube.com", "youtu.be", "ytimg.com", "googlevideo.com",
	},
	"facebook": {
		"facebook.com", "fb.com", "fbcdn.net", "instagram.com", "cdninstagram.com",
		"whatsapp.com", "wa.me", "messenger.com",
	},
	"meta": {
		"facebook.com", "fb.com", "fbcdn.net", "instagram.com", "cdninstagram.com",
	},
	"twitter": {
		"twitter.com", "x.com", "t.co", "twimg.com",
	},
	"discord": {
		"discord.com", "discord.gg", "discordapp.com", "discordapp.net", "discordcdn.com",
	},
	"telegram": {
		"telegram.org", "t.me", "telegra.ph", "tdesktop.com",
	},
	"tiktok": {
		"tiktok.com", "tiktokcdn.com", "byteoversea.com", "musical.ly",
	},
	"reddit": {
		"reddit.com", "redd.it", "redditmedia.com", "redditstatic.com",
	},
	"linkedin": {
		"linkedin.com", "licdn.com",
	},
	"spotify": {
		"spotify.com", "scdn.co", "spotifycdn.com",
	},
	"netflix": {
		"netflix.com", "nflxvideo.net", "nflximg.net", "nflxso.net",
	},
	"openai": {
		"openai.com", "chatgpt.com", "oaistatic.com", "oaiusercontent.com",
	},
	"category-ads-all": {}, // empty — we treat as "block-ish" pattern, but unknown
	"private":          {}, // private IP space — domain match unlikely
	"cn": { // tiny sample
		"baidu.com", "qq.com", "weibo.com", "taobao.com", "tmall.com",
		"jd.com", "alipay.com",
	},
}

// ResolveOutboundResult is what the resolver returns for one host.
type ResolveOutboundResult struct {
	OutboundTag string `json:"outboundTag"` // resolved tag, or empty if no rule matched
	RuleIndex   int    `json:"ruleIndex"`   // index in routing.rules, or -1
	MatchedBy   string `json:"matchedBy"`   // pattern that matched, e.g. "domain:vk.com" or "geosite:ru"
	Confident   bool   `json:"confident"`   // false if matched only via hardcoded geosite hints
}

// ResolveOutbound walks routing.rules and returns the outbound tag that the
// given host would be routed to. If no rule matches, returns the active
// outbound (the "default" outbound for the config).
func ResolveOutbound(host string, info *ConfigInfo) ResolveOutboundResult {
	host = strings.ToLower(strings.TrimSpace(host))
	if info == nil || host == "" {
		return ResolveOutboundResult{RuleIndex: -1}
	}

	for i, rule := range info.Rules {
		for _, pattern := range rule.Domain {
			match, confident := matchDomainPattern(host, pattern)
			if match {
				return ResolveOutboundResult{
					OutboundTag: rule.OutboundTag,
					RuleIndex:   i,
					MatchedBy:   pattern,
					Confident:   confident,
				}
			}
		}
	}

	// No rule matched — return the *default* outbound (first in the array, per
	// xray semantics), NOT the "primary" proxy.
	return ResolveOutboundResult{
		OutboundTag: info.Default,
		RuleIndex:   -1,
		MatchedBy:   "default",
		Confident:   true,
	}
}

// matchDomainPattern returns (matches, confident).
//   - "domain:foo.com" → exact or subdomain match (confident)
//   - "keyword:foo"    → substring match (confident)
//   - "regexp:..."     → regex match (confident)
//   - "geosite:cat"    → checked against the hardcoded hints map (NOT confident)
//   - bare "foo.com"   → subdomain match (confident)
func matchDomainPattern(host, pattern string) (bool, bool) {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	switch {
	case strings.HasPrefix(pattern, "geosite:"):
		cat := strings.TrimPrefix(pattern, "geosite:")
		domains, ok := geositeHints[cat]
		if !ok {
			return false, false
		}
		for _, d := range domains {
			if subdomainMatch(host, d) {
				return true, false
			}
		}
		return false, false
	case strings.HasPrefix(pattern, "domain:"):
		return subdomainMatch(host, strings.TrimPrefix(pattern, "domain:")), true
	case strings.HasPrefix(pattern, "full:"):
		return host == strings.TrimPrefix(pattern, "full:"), true
	case strings.HasPrefix(pattern, "keyword:"):
		return strings.Contains(host, strings.TrimPrefix(pattern, "keyword:")), true
	case strings.HasPrefix(pattern, "regexp:"):
		re, err := regexp.Compile(strings.TrimPrefix(pattern, "regexp:"))
		if err != nil {
			return false, false
		}
		return re.MatchString(host), true
	default:
		return subdomainMatch(host, pattern), true
	}
}

func subdomainMatch(host, base string) bool {
	if host == base {
		return true
	}
	return strings.HasSuffix(host, "."+base)
}
