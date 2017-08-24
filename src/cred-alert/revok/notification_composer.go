package revok

import (
	"context"

	"cloud.google.com/go/trace"
	"code.cloudfoundry.org/lager"

	"cred-alert/db"
	"cred-alert/notifications"
)

type NotificationComposer struct {
	repositoryRepository db.RepositoryRepository
	router               notifications.Router
	scanner              Scanner
}

func NewNotificationComposer(
	repositoryRepository db.RepositoryRepository,
	router notifications.Router,
	scanner Scanner,
) *NotificationComposer {
	return &NotificationComposer{
		repositoryRepository: repositoryRepository,
		router:               router,
		scanner:              scanner,
	}
}

func (n *NotificationComposer) ScanAndNotify(
	ctx context.Context,
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

	scanSpan := trace.FromContext(ctx).NewChild("scanning")
	scanSpan.SetLabel("branch", branch)
	credentials, err := n.scanner.Scan(logger, owner, repository, scannedOids, branch, startSHA, stopSHA)
	if err != nil {
		return err
	}
	scanSpan.Finish()

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
		err = n.router.Deliver(ctx, logger, batch)
		if err != nil {
			logger.Error("failed", err)
			return err
		}
	}

	return nil
}
