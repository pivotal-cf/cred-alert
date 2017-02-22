package revok

import (
	"code.cloudfoundry.org/lager"

	"cred-alert/db"
	"cred-alert/notifications"
)

//go:generate counterfeiter . NotificationComposer

type NotificationComposer interface {
	ScanAndNotify(lager.Logger, string, string, map[string]struct{}, string, string, string) error
}

type notificationComposer struct {
	repositoryRepository db.RepositoryRepository
	router               notifications.Router
	scanner              Scanner
}

func NewNotificationComposer(
	repositoryRepository db.RepositoryRepository,
	router notifications.Router,
	scanner Scanner,
) NotificationComposer {
	return &notificationComposer{
		repositoryRepository: repositoryRepository,
		router:               router,
		scanner:              scanner,
	}
}

func (n *notificationComposer) ScanAndNotify(
	logger lager.Logger,
	owner string,
	repository string,
	scannedOids map[string]struct{},
	branch string,
	startSHA string,
	stopSHA string,
) error {
	dbRepository, err := n.repositoryRepository.MustFind(owner, repository)
	if err != nil {
		logger.Error("failed-to-find-db-repo", err)
		return err
	}

	credentials, err := n.scanner.Scan(logger, owner, repository, scannedOids, branch, startSHA, stopSHA)
	if err != nil {
		return err
	}

	var batch []notifications.Notification

	for _, credential := range credentials {
		batch = append(batch, notifications.Notification{
			Owner:      credential.Owner,
			Repository: credential.Repository,
			Branch:     branch,
			SHA:        credential.SHA,
			Path:       credential.Path,
			LineNumber: credential.LineNumber,
			Private:    dbRepository.Private,
		})
	}

	if len(batch) > 0 {
		err = n.router.Deliver(logger, batch)
		if err != nil {
			logger.Error("failed", err)
			return err
		}
	}

	return nil
}
