package db

import (
	"code.cloudfoundry.org/lager"
	"github.com/jinzhu/gorm"
)

//go:generate counterfeiter . FailedMessageRepository

type FailedMessageRepository interface {
	GetFailedMessages(lager.Logger) ([]FailedMessage, error)
	GetDeadLetters(lager.Logger) ([]FailedMessage, error)
	RegisterFailedMessage(lager.Logger, string) (int, error)
	MarkFailedMessageAsDead(lager.Logger, string) error
	RemoveFailedMessage(lager.Logger, string) error
}

type failedMessageRepository struct {
	db *gorm.DB
}

func NewFailedMessageRepository(db *gorm.DB) *failedMessageRepository {
	return &failedMessageRepository{db: db}
}

func (r *failedMessageRepository) GetFailedMessages(logger lager.Logger) ([]FailedMessage, error) {
	var failedMessages []FailedMessage

	r.db.Find(&failedMessages)

	return failedMessages, nil
}

func (r *failedMessageRepository) GetDeadLetters(logger lager.Logger) ([]FailedMessage, error) {
	var deadLetters []FailedMessage

	r.db.Where("dead_lettered=true").Find(&deadLetters)

	return deadLetters, nil
}

func (r *failedMessageRepository) RegisterFailedMessage(logger lager.Logger, messageID string) (int, error) {
	logger = logger.Session("register-failed-message", lager.Data{
		"pubsub-message-id": messageID,
	})

	logger.Debug("starting")

	tx, err := r.db.DB().Begin()
	if err != nil {
		return 0, err
	}

	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO failed_messages (pubsub_message_id, retries) VALUES (?, 1)
	  ON DUPLICATE KEY UPDATE retries = retries + 1
	`, messageID)
	if err != nil {
		logger.Error("failed-to-increment-retries", err)
		return 0, err
	}

	var retries int

	err = tx.QueryRow(`
	  SELECT retries
		FROM failed_messages
		WHERE pubsub_message_id = ?
	`, messageID).Scan(&retries)

	if err != nil {
		logger.Error("failed-to-get-retry-count", err)
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		logger.Error("failed-to-commit", err)
		return 0, err
	}

	logger.Debug("done")
	return retries, nil
}

func (r *failedMessageRepository) MarkFailedMessageAsDead(logger lager.Logger, messageID string) error {
	tx, err := r.db.DB().Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()
	_, err = tx.Exec(`
		UPDATE failed_messages
		SET dead_lettered = TRUE
	  WHERE pubsub_message_id = ? LIMIT 1
	`, messageID)
	if err != nil {
		logger.Error("failed-to-mark-as-dead", err)
		return err
	}

	if err := tx.Commit(); err != nil {
		logger.Error("failed-to-commit", err)
		return err
	}

	return nil
}

func (r *failedMessageRepository) RemoveFailedMessage(logger lager.Logger, messageID string) error {
	tx, err := r.db.DB().Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec(`
		DELETE FROM failed_messages
		WHERE pubsub_message_id = ? LIMIT 1
	`, messageID)

	if err != nil {
		logger.Error("failed-to-delete-failed-message", err)
		return err
	}

	if err := tx.Commit(); err != nil {
		logger.Error("failed-to-commit", err)
		return err
	}

	return nil
}
