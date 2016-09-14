package revok

import (
	"cred-alert/db"
	"cred-alert/metrics"

	"code.cloudfoundry.org/lager"
)

const successMetric = "revok.success_jobs"
const failedMetric = "revok.failed_jobs"

func finishScan(logger lager.Logger, scan db.ActiveScan, success, failed metrics.Counter) {
	err := scan.Finish()
	if err != nil {
		logger.Error("failed-to-finish-scan", err)
		failed.Inc(logger)
	} else {
		success.Inc(logger)
	}
}
