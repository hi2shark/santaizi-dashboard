package mygin

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/naiba/nezha/model"
	"github.com/naiba/nezha/service/singleton"
)

var adminPage = map[string]bool{
	"/server":                 true,
	"/server/offline-history": true,
	"/monitor":                true,
	"/setting":                true,
	"/notification":           true,
	"/ddns":                   true,
	"/nat":                    true,
	"/cron":                   true,
	"/api":                    true,
}

var dashboardLangMessageIDs = []string{
	"Add",
	"AddRule",
	"AccessAndVisibility",
	"AlarmRule",
	"AllIncludedOnlySpecificServersAreNotAlerted",
	"AllIncludedOnlySpecificServersAreNotExecuted",
	"AllIncludedOnlySpecificServersAreNotRequest",
	"Amount",
	"AmountPlaceholder",
	"AutoRenewal",
	"BatchDeleteServer",
	"Bandwidth",
	"BandwidthPlaceholder",
	"BindHostname",
	"BillingCycle",
	"BillingCyclePlaceholder",
	"BillingInfo",
	"BuyBtnIcon",
	"BuyBtnIconPlaceholder",
	"BuyBtnText",
	"BuyBtnTextPlaceholder",
	"Cancel",
	"ClickToCopy",
	"Command",
	"Confirm",
	"ConfirmCloseModal",
	"ConfirmToDeleteServer",
	"ConfirmToResetSecret",
	"Coverage",
	"CronTask",
	"CycleInterval",
	"CycleStart",
	"CycleUnit",
	"DDNS",
	"DDNSAccessID",
	"DDNSAccessSecret",
	"DDNSDomains",
	"DDNSProfiles",
	"DDNSProvider",
	"DisplayIndex",
	"DoNotSendTestMessages",
	"Duration",
	"Edit",
	"Enable",
	"EnableDDNS",
	"EnableFailureNotification",
	"EnableIPv4",
	"EnableIPv6",
	"EnableLatencyNotification",
	"EnableShowInService",
	"EnableTriggerTask",
	"EndDate",
	"ExecuteByTriggerServer",
	"Extra",
	"ExtraPlaceholder",
	"FailTriggerTasks",
	"FinalJSONPreview",
	"Flag",
	"FlagPlaceholder",
	"HideForGuest",
	"IgnoreAllAndExecuteOnlyThroughSpecificServers",
	"IgnoreAllOnlyAlertSpecificServers",
	"IgnoreAllRequestOnlyThroughSpecificServers",
	"InputServerGroupName",
	"InternalNote",
	"IntroductionOfMonitor",
	"Latitude",
	"LatLng",
	"Loading",
	"LocalService",
	"LocationCode",
	"LocationCodePlaceholder",
	"LocationLabel",
	"Longitude",
	"MaxLatency",
	"MaxRetries",
	"MaxThreshold",
	"MinLatency",
	"MinThreshold",
	"ModeAlwaysTrigger",
	"ModeOnetimeTrigger",
	"ModifiedSuccessfully",
	"Name",
	"NoData",
	"NoMatch",
	"Note",
	"NotificationMethod",
	"NotificationMethodGroup",
	"NotificationTriggerMode",
	"NetworkRoute",
	"NetworkRoutePlaceholder",
	"OrderLink",
	"OrderLinkPlaceholder",
	"PleaseEnterValidJSON",
	"PleaseSelect",
	"PlanInfo",
	"PublicNote",
	"PushSuccessMessages",
	"RawJSON",
	"RecoverTriggerTasks",
	"RemoveRule",
	"RequestMethod",
	"RequestType",
	"ResetSecret",
	"RuleType",
	"RuleTypeDesc_cpu",
	"RuleTypeDesc_disk",
	"RuleTypeDesc_load1",
	"RuleTypeDesc_load15",
	"RuleTypeDesc_load5",
	"RuleTypeDesc_memory",
	"RuleTypeDesc_net_all_speed",
	"RuleTypeDesc_net_in_speed",
	"RuleTypeDesc_net_out_speed",
	"RuleTypeDesc_offline",
	"RuleTypeDesc_process_count",
	"RuleTypeDesc_swap",
	"RuleTypeDesc_tcp_conn_count",
	"RuleTypeDesc_temperature_max",
	"RuleTypeDesc_transfer_all",
	"RuleTypeDesc_transfer_all_cycle",
	"RuleTypeDesc_transfer_in",
	"RuleTypeDesc_transfer_in_cycle",
	"RuleTypeDesc_transfer_out",
	"RuleTypeDesc_transfer_out_cycle",
	"RuleTypeDesc_udp_conn_count",
	"Scheduler",
	"Secret",
	"Server",
	"ServerBasicInfo",
	"ServerGroup",
	"Settings",
	"Slogan",
	"SloganPlaceholder",
	"SpecificServers",
	"SslExpirationOrChange",
	"StartDate",
	"Tag",
	"Target",
	"TaskType",
	"TrafficBidirectional",
	"TrafficMaxInOut",
	"TrafficOutboundOnly",
	"TrafficType",
	"TrafficVol",
	"TrafficVolPlaceholder",
	"TriggerTask",
	"Type",
	"UnlimitedDuration",
	"VerifySSL",
	"VisualEditor",
	"WebhookHeaders",
	"WebhookMethod",
	"WebhookRequestBody",
	"WebhookRequestType",
	"WebhookURL",
}

func localize(messageID string) string {
	return singleton.Localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: messageID})
}

func dashboardLang() map[string]string {
	lang := make(map[string]string, len(dashboardLangMessageIDs)+3)
	for _, messageID := range dashboardLangMessageIDs {
		lang[messageID] = localize(messageID)
	}
	lang["Cron"] = localize("ScheduledTasks")
	lang["Monitor"] = localize("ServicesManagement")
	lang["Notification"] = localize("NotificationMethod")
	return lang
}

func CommonEnvironment(c *gin.Context, data map[string]interface{}) gin.H {
	matchedPath, ok := data["MatchedPath"].(string)
	if !ok || matchedPath == "" {
		if path, exists := c.Get("MatchedPath"); exists {
			matchedPath, _ = path.(string)
		}
		if matchedPath == "" {
			matchedPath = c.FullPath()
		}
		if matchedPath == "" && c.Request != nil && c.Request.URL != nil {
			matchedPath = c.Request.URL.Path
		}
	}
	data["MatchedPath"] = matchedPath
	data["Version"] = singleton.Version
	data["CSRFToken"] = CSRFToken(c)
	_, isAuthorized := c.Get(model.CtxKeyAuthorizedUser)
	if isAuthorized {
		data["Conf"] = singleton.Conf
	} else {
		data["Conf"] = model.PublicConfig{
			Site: model.PublicSiteConfig{
				Brand:               singleton.Conf.Site.Brand,
				Theme:               singleton.Conf.Site.Theme,
				DashboardTheme:      singleton.Conf.Site.DashboardTheme,
				CustomCode:          singleton.Conf.Site.CustomCode,
				CustomCodeDashboard: singleton.Conf.Site.CustomCodeDashboard,
			},
			Language:                        singleton.Conf.Language,
			MaxTCPPingValue:                 singleton.Conf.MaxTCPPingValue,
			DisableSwitchTemplateInFrontend: singleton.Conf.DisableSwitchTemplateInFrontend,
		}
	}
	data["Themes"] = model.Themes
	data["CustomCode"] = singleton.Conf.Site.CustomCode
	data["CustomCodeDashboard"] = singleton.Conf.Site.CustomCodeDashboard
	// 是否是管理页面
	isAdminPage := adminPage[matchedPath]
	data["IsAdminPage"] = isAdminPage
	if _, ok := data["IsDashboardPage"]; !ok {
		data["IsDashboardPage"] = isAdminPage || matchedPath == "/login"
	}
	// 站点标题
	if t, has := data["Title"]; !has {
		data["Title"] = singleton.Conf.Site.Brand
	} else {
		data["Title"] = fmt.Sprintf("%s - %s", t, singleton.Conf.Site.Brand)
	}
	u, ok := c.Get(model.CtxKeyAuthorizedUser)
	if ok {
		data["Admin"] = u
	}
	data["LANG"] = dashboardLang()
	return data
}

func RecordPath(c *gin.Context) {
	url := c.Request.URL.Path
	for _, p := range c.Params {
		url = strings.Replace(url, p.Value, ":"+p.Key, 1)
	}
	c.Set("MatchedPath", url)
}
