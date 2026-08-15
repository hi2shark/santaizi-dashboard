package singleton

import (
	"log"
	"time"

	trafficservice "github.com/hi2shark/santaizi-dashboard/service/traffic"
)

func startTrafficPolicyEvaluator() {
	_, err := Cron.AddFunc("@every 60s", func() {
		if err := trafficservice.EvaluateAll(DB, time.Now(), func(tag, message string, serverID uint64) {
			if tag == "" {
				tag = "default"
			}
			ServerLock.RLock()
			server := ServerList[serverID]
			ServerLock.RUnlock()
			SendNotification(tag, message, NotificationMuteLabel.TrafficWarning(serverID), server)
		}); err != nil {
			log.Printf("SANTAIZI>> traffic policy evaluation failed: %v", err)
		}
	})
	if err != nil {
		panic(err)
	}
}
