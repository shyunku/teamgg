package models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
	"time"
)

const (
	ReplayAnalysisStatusQueued    = "queued"
	ReplayAnalysisStatusUploading = "uploading"
	ReplayAnalysisStatusAnalyzing = "analyzing"
	ReplayAnalysisStatusCompleted = "completed"
	ReplayAnalysisStatusFailed    = "failed"
)

type CustomGameReplayAnalysisDAO struct {
	Id                 string     `db:"id" json:"id"`
	CustomGameConfigId string     `db:"custom_game_config_id" json:"customGameId"`
	CreatorUid         string     `db:"creator_uid" json:"-"`
	RequestId          *string    `db:"request_id" json:"requestId,omitempty"`
	FileName           string     `db:"file_name" json:"fileName"`
	FileSize           int64      `db:"file_size" json:"fileSize"`
	Status             string     `db:"status" json:"status"`
	Stage              string     `db:"stage" json:"stage"`
	Progress           int        `db:"progress" json:"progress"`
	Analysis           *string    `db:"analysis" json:"analysis,omitempty"`
	Model              *string    `db:"model" json:"model,omitempty"`
	ErrorMessage       *string    `db:"error_message" json:"error,omitempty"`
	CreatedAt          time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt          time.Time  `db:"updated_at" json:"updatedAt"`
	CompletedAt        *time.Time `db:"completed_at" json:"completedAt,omitempty"`
}

func EnsureCustomGameReplayAnalysisSchema(database db.Context) error {
	_, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS custom_game_replay_analyses (
			id VARCHAR(36) NOT NULL,
			custom_game_config_id VARCHAR(255) NOT NULL,
			creator_uid VARCHAR(255) NOT NULL,
			request_id VARCHAR(64) NULL,
			file_name VARCHAR(255) NOT NULL,
			file_size BIGINT NOT NULL,
			status VARCHAR(24) NOT NULL,
			stage VARCHAR(255) NOT NULL,
			progress INT NOT NULL DEFAULT 0,
			analysis LONGTEXT NULL,
			model VARCHAR(100) NULL,
			error_message TEXT NULL,
			created_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			completed_at DATETIME(6) NULL,
			PRIMARY KEY (id),
			KEY custom_game_replay_analyses_config_created_index (custom_game_config_id, created_at),
			KEY custom_game_replay_analyses_status_index (status),
			CONSTRAINT custom_game_replay_analyses_config_fk
				FOREIGN KEY (custom_game_config_id) REFERENCES custom_game_configurations (id)
				ON UPDATE CASCADE ON DELETE CASCADE,
			CONSTRAINT custom_game_replay_analyses_creator_fk
				FOREIGN KEY (creator_uid) REFERENCES users (uid)
				ON UPDATE CASCADE ON DELETE CASCADE
		) ENGINE=InnoDB
	`)
	return err
}

func (analysis *CustomGameReplayAnalysisDAO) Insert(database db.Context) error {
	_, err := database.Exec(`
		INSERT INTO custom_game_replay_analyses (
			id, custom_game_config_id, creator_uid, request_id, file_name, file_size,
			status, stage, progress, analysis, model, error_message,
			created_at, updated_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, analysis.Id, analysis.CustomGameConfigId, analysis.CreatorUid, analysis.RequestId,
		analysis.FileName, analysis.FileSize, analysis.Status, analysis.Stage, analysis.Progress,
		analysis.Analysis, analysis.Model, analysis.ErrorMessage, analysis.CreatedAt,
		analysis.UpdatedAt, analysis.CompletedAt)
	return err
}

func (analysis *CustomGameReplayAnalysisDAO) Update(database db.Context) error {
	_, err := database.Exec(`
		UPDATE custom_game_replay_analyses
		SET request_id = ?, status = ?, stage = ?, progress = ?, analysis = ?, model = ?,
			error_message = ?, updated_at = ?, completed_at = ?
		WHERE id = ?
	`, analysis.RequestId, analysis.Status, analysis.Stage, analysis.Progress, analysis.Analysis,
		analysis.Model, analysis.ErrorMessage, analysis.UpdatedAt, analysis.CompletedAt, analysis.Id)
	return err
}

func GetCustomGameReplayAnalysisById(database db.Context, id string) (*CustomGameReplayAnalysisDAO, bool, error) {
	var analysis CustomGameReplayAnalysisDAO
	err := database.Get(&analysis, `SELECT * FROM custom_game_replay_analyses WHERE id = ?`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &analysis, true, nil
}

func GetCustomGameReplayAnalysisByIdForUpdate(database db.Context, id string) (*CustomGameReplayAnalysisDAO, bool, error) {
	var analysis CustomGameReplayAnalysisDAO
	err := database.Get(&analysis, `SELECT * FROM custom_game_replay_analyses WHERE id = ? FOR UPDATE`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &analysis, true, nil
}

func GetCustomGameReplayAnalysesByConfigId(database db.Context, configId string) ([]CustomGameReplayAnalysisDAO, error) {
	analyses := make([]CustomGameReplayAnalysisDAO, 0)
	err := database.Select(&analyses, `
		SELECT id, custom_game_config_id, creator_uid, request_id, file_name, file_size,
		       status, stage, progress, NULL AS analysis, model, error_message,
		       created_at, updated_at, completed_at
		FROM custom_game_replay_analyses
		WHERE custom_game_config_id = ?
		ORDER BY created_at DESC
		LIMIT 50
	`, configId)
	return analyses, err
}

func HasRunningCustomGameReplayAnalysis(database db.Context, configId string) (bool, error) {
	var count int
	err := database.Get(&count, `
		SELECT COUNT(*) FROM custom_game_replay_analyses
		WHERE custom_game_config_id = ? AND status IN (?, ?, ?)
	`, configId, ReplayAnalysisStatusQueued, ReplayAnalysisStatusUploading, ReplayAnalysisStatusAnalyzing)
	return count > 0, err
}

func FailStaleCustomGameReplayAnalyses(database db.Context, configId string, cutoff, now time.Time) (int64, error) {
	result, err := database.Exec(`
		UPDATE custom_game_replay_analyses
		SET status = ?, stage = ?, error_message = ?, updated_at = ?, completed_at = ?
		WHERE custom_game_config_id = ?
		  AND status IN (?, ?, ?)
		  AND updated_at < ?
	`, ReplayAnalysisStatusFailed, "분석 시간 초과", "분석 서버의 응답이 없어 작업이 종료되었습니다.", now, now,
		configId, ReplayAnalysisStatusQueued, ReplayAnalysisStatusUploading, ReplayAnalysisStatusAnalyzing, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func DeleteCustomGameReplayAnalysis(database db.Context, id string) error {
	_, err := database.Exec(`DELETE FROM custom_game_replay_analyses WHERE id = ?`, id)
	return err
}
