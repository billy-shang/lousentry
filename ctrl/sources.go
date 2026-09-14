package ctrl

import (
	"fmt"
	"strings"

	"lousentry/grab"
)

// SourceOption 前端展示用的数据源元信息。
type SourceOption struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Link        string `json:"link"`
}

// KnownSources 控制台可选的全部漏洞源。
func KnownSources() []SourceOption {
	return []SourceOption{
		{ID: "avd", Name: "avd", DisplayName: "阿里云漏洞库", Link: "https://avd.aliyun.com/high-risk/list"},
		{ID: "chaitin", Name: "chaitin", DisplayName: "长亭漏洞库", Link: "https://stack.chaitin.com/vuldb/index"},
		{ID: "oscs", Name: "oscs", DisplayName: "OSCS开源安全情报预警", Link: "https://www.oscs1024.com/cm"},
		{ID: "ti", Name: "ti", DisplayName: "奇安信威胁情报中心", Link: "https://ti.qianxin.com/"},
		{ID: "threatbook", Name: "threatbook", DisplayName: "微步在线研究响应中心", Link: "https://x.threatbook.com/v5/vulIntelligence"},
		{ID: "seebug", Name: "seebug", DisplayName: "知道创宇Seebug漏洞库", Link: "https://www.seebug.org/"},
		{ID: "venustech", Name: "venustech", DisplayName: "启明星辰漏洞通告", Link: "https://www.venustech.com.cn/new_type/aqtg/"},
		{ID: "kev", Name: "kev", DisplayName: "CISA KEV", Link: "https://www.cisa.gov/known-exploited-vulnerabilities-catalog"},
		{ID: "struts2", Name: "struts2", DisplayName: "Apache Struts2 Security Bulletins", Link: "https://cwiki.apache.org/confluence/display/WW/Security+Bulletins"},
	}
}

// NormalizeSource 将历史别名统一成前端使用的 id。
func NormalizeSource(id string) string {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "nox", "ti", "qianxin", "qianxin-ti":
		return "ti"
	case "struts2", "structs2":
		return "struts2"
	default:
		return strings.ToLower(strings.TrimSpace(id))
	}
}

// BuildGrabbers 按配置创建采集器。
func BuildGrabbers(sources []string) ([]grab.Grabber, error) {
	var grabs []grab.Grabber
	for _, part := range sources {
		part = NormalizeSource(part)
		if part == "" {
			continue
		}
		switch part {
		case "chaitin":
			grabs = append(grabs, grab.NewChaitinCrawler())
		case "avd":
			grabs = append(grabs, grab.NewAVDCrawler())
		case "ti":
			grabs = append(grabs, grab.NewTiCrawler())
		case "oscs":
			grabs = append(grabs, grab.NewOSCSCrawler())
		case "seebug":
			grabs = append(grabs, grab.NewSeebugCrawler())
		case "threatbook":
			grabs = append(grabs, grab.NewThreatBookCrawler())
		case "struts2":
			grabs = append(grabs, grab.NewStruts2Crawler())
		case "kev":
			grabs = append(grabs, grab.NewKEVCrawler())
		case "venustech":
			grabs = append(grabs, grab.NewVenustechCrawler())
		default:
			return nil, fmt.Errorf("invalid grab source %s", part)
		}
	}
	if len(grabs) == 0 {
		return nil, fmt.Errorf("至少需要启用一个漏洞数据源")
	}
	return grabs, nil
}

// SourceFromURL 根据漏洞来源链接反推数据源。
func SourceFromURL(from string) SourceOption {
	u := strings.ToLower(from)
	switch {
	case strings.Contains(u, "avd.aliyun.com") || strings.Contains(u, "aliyun.com"):
		return sourceByID("avd")
	case strings.Contains(u, "chaitin"):
		return sourceByID("chaitin")
	case strings.Contains(u, "oscs1024") || strings.Contains(u, "oscs"):
		return sourceByID("oscs")
	case strings.Contains(u, "qianxin") || strings.Contains(u, "ti.qianxin"):
		return sourceByID("ti")
	case strings.Contains(u, "threatbook"):
		return sourceByID("threatbook")
	case strings.Contains(u, "seebug"):
		return sourceByID("seebug")
	case strings.Contains(u, "venustech"):
		return sourceByID("venustech")
	case strings.Contains(u, "cisa.gov") || strings.Contains(u, "known-exploited"):
		return sourceByID("kev")
	case strings.Contains(u, "struts") || strings.Contains(u, "cwiki.apache.org"):
		return sourceByID("struts2")
	default:
		return SourceOption{ID: "", Name: "", DisplayName: from, Link: from}
	}
}

func sourceByID(id string) SourceOption {
	for _, s := range KnownSources() {
		if s.ID == id {
			return s
		}
	}
	return SourceOption{ID: id, Name: id, DisplayName: id}
}

func sourceMatchKeyword(from, sourceID string) bool {
	if sourceID == "" {
		return true
	}
	info := SourceFromURL(from)
	return info.ID == NormalizeSource(sourceID)
}
