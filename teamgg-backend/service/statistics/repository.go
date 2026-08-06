package statistics

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	log "github.com/shyunku-libraries/go-logger"
	"io"
	"math"
	"os"
	"path"
	"strings"
	"team.gg-server/core"
	"team.gg-server/libs/db"
	"team.gg-server/models"
	"team.gg-server/util"
	"time"
)

const StatisticsDataPath = "datafiles/statistics"

var (
	StatisticsDB                  *db.Database                        = nil
	ChampionDetailStatisticsRepo  *ChampionDetailStatisticsRepository = nil
	TierStatisticsRepo            *TierStatisticsRepository           = nil
	MasteryStatisticsRepo         *MasteryStatisticsRepository        = nil
	ErrStatisticsCollectionLocked                                     = errors.New("statistics collection is running on another instance")
)

type cancelableDatabase struct {
	ctx  context.Context
	conn *sqlx.Conn
}

func (database cancelableDatabase) Exec(query string, args ...interface{}) (sql.Result, error) {
	return database.conn.ExecContext(database.ctx, query, args...)
}

func (database cancelableDatabase) Get(dest interface{}, query string, args ...interface{}) error {
	return database.conn.GetContext(database.ctx, dest, query, args...)
}

func (database cancelableDatabase) Select(dest interface{}, query string, args ...interface{}) error {
	return database.conn.SelectContext(database.ctx, dest, query, args...)
}

func (database cancelableDatabase) Rebind(query string) string {
	return database.conn.Rebind(query)
}

type RunnerConfig struct {
	Enabled        bool
	Period         time.Duration
	InitialDelay   time.Duration
	RetryDelay     time.Duration
	LockRetryDelay time.Duration
	LockTimeout    time.Duration
}

type Statistics[T any] interface {
	key() string
	Period() time.Duration
	Loop(context.Context)
	Collect() (*T, error)
	Save() error
	Load() (*T, error)
}

func InitializeStatisticRepos() error {
	retryDelay, err := durationEnvironment("STATISTICS_RETRY_DELAY", 5*time.Minute, false)
	if err != nil {
		return err
	}
	lockTimeout, err := durationEnvironment("STATISTICS_LOCK_TIMEOUT", time.Second, true)
	if err != nil {
		return err
	}
	lockRetryDelay, err := durationEnvironment("STATISTICS_LOCK_RETRY_DELAY", 15*time.Second, false)
	if err != nil {
		return err
	}

	championPeriod := 24 * time.Hour
	tierPeriod := 12 * time.Hour
	masteryPeriod := 12 * time.Hour
	if !core.IsProduction {
		championPeriod = time.Hour
		tierPeriod = time.Hour
		masteryPeriod = time.Hour
	}

	championConfig, err := repositoryConfig(
		"CHAMPION_DETAIL",
		championPeriod,
		10*time.Minute,
		retryDelay,
		lockRetryDelay,
		lockTimeout,
	)
	if err != nil {
		return err
	}
	tierConfig, err := repositoryConfig(
		"TIER",
		tierPeriod,
		30*time.Second,
		retryDelay,
		lockRetryDelay,
		lockTimeout,
	)
	if err != nil {
		return err
	}
	masteryConfig, err := repositoryConfig(
		"MASTERY",
		masteryPeriod,
		2*time.Minute,
		retryDelay,
		lockRetryDelay,
		lockTimeout,
	)
	if err != nil {
		return err
	}

	ChampionDetailStatisticsRepo = NewChampionDetailStatisticsRepository(championConfig)
	TierStatisticsRepo = NewTierStatisticsRepository(tierConfig)
	MasteryStatisticsRepo = NewMasteryStatisticsRepository(masteryConfig)
	return nil
}

func repositoryConfig(name string, period, initialDelay, retryDelay, lockRetryDelay, lockTimeout time.Duration) (RunnerConfig, error) {
	prefix := "STATISTICS_" + name
	parsedPeriod, err := durationEnvironment(prefix+"_PERIOD", period, false)
	if err != nil {
		return RunnerConfig{}, err
	}
	parsedInitialDelay, err := durationEnvironment(prefix+"_INITIAL_DELAY", initialDelay, true)
	if err != nil {
		return RunnerConfig{}, err
	}
	return RunnerConfig{
		Enabled:        boolEnvironment(prefix + "_ENABLED"),
		Period:         parsedPeriod,
		InitialDelay:   parsedInitialDelay,
		RetryDelay:     retryDelay,
		LockRetryDelay: lockRetryDelay,
		LockTimeout:    lockTimeout,
	}, nil
}

func durationEnvironment(key string, fallback time.Duration, allowZero bool) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a Go duration such as 30s, 15m or 12h: %w", key, err)
	}
	if value < 0 || (!allowZero && value == 0) {
		return 0, fmt.Errorf("%s must be %s", key, map[bool]string{true: "zero or greater", false: "greater than zero"}[allowZero])
	}
	return value, nil
}

func boolEnvironment(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "on", "yes":
		return true
	default:
		return false
	}
}

func runLoop(
	ctx context.Context,
	key string,
	config RunnerConfig,
	collect func(context.Context) (time.Duration, error),
) {
	if !config.Enabled {
		log.Infof("Statistics %s loop is disabled", key)
		return
	}
	log.Infof(
		"Statistics %s loop enabled: period=%s initialDelay=%s retryDelay=%s lockRetryDelay=%s lockTimeout=%s",
		key,
		config.Period,
		config.InitialDelay,
		config.RetryDelay,
		config.LockRetryDelay,
		config.LockTimeout,
	)

	if !waitContext(ctx, config.InitialDelay) {
		return
	}
	for {
		delay := config.Period
		scheduledDelay, err := collect(ctx)
		if err != nil {
			if errors.Is(err, ErrStatisticsCollectionLocked) {
				delay = config.LockRetryDelay
				log.Infof("Statistics %s is being collected by another instance; retrying in %s", key, delay)
			} else if !errors.Is(err, context.Canceled) {
				delay = config.RetryDelay
				log.Errorf("Statistics %s collection failed; retrying in %s: %v", key, delay, err)
			}
		} else if scheduledDelay > 0 {
			delay = scheduledDelay
		}
		if !waitContext(ctx, delay) {
			return
		}
	}
}

func waitContext(ctx context.Context, duration time.Duration) bool {
	if duration <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func collectCoordinated[T any](
	ctx context.Context,
	key string,
	config RunnerConfig,
	collector func(db.Context) (*T, error),
	setCache func(*T),
) (*T, time.Duration, error) {
	if StatisticsDB == nil {
		return nil, 0, errors.New("statistics database is not initialized")
	}
	conn, err := StatisticsDB.Connx(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Close()
	database := cancelableDatabase{ctx: ctx, conn: conn}

	// CREATE TEMPORARY TABLE ... SELECT may take shared record locks under
	// InnoDB's default REPEATABLE READ isolation and block DataExplorer writes.
	// READ COMMITTED makes these analytical reads non-locking while the
	// dedicated statistics connection still keeps all temporary tables scoped.
	if _, err := database.Exec("SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED"); err != nil {
		return nil, 0, err
	}

	// One global lock serializes all heavy statistics scans across every server
	// instance. Per-repository locks would still allow champion, tier and
	// mastery aggregations to compete for the same database at once.
	lockName := "teamgg_statistics_global"
	lockSeconds := int(math.Ceil(config.LockTimeout.Seconds()))
	var acquired sql.NullInt64
	if err := conn.GetContext(ctx, &acquired, "SELECT GET_LOCK(?, ?)", lockName, lockSeconds); err != nil {
		return nil, 0, err
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		return nil, 0, ErrStatisticsCollectionLocked
	}
	defer func() {
		var released sql.NullInt64
		if err := conn.GetContext(context.Background(), &released, "SELECT RELEASE_LOCK(?)", lockName); err != nil {
			log.Warnf("Failed to release statistics lock %s: %v", lockName, err)
		}
	}()
	// Check freshness only after acquiring the lock. This prevents every server
	// instance from recomputing the same snapshot after the first one finishes.
	snapshot, err := models.GetStatisticsSnapshot(database, key, config.Period)
	if err != nil {
		return nil, 0, err
	}
	if snapshot != nil && snapshot.Fresh {
		var cached T
		payload, err := decodeSharedPayload(snapshot.Payload)
		if err != nil {
			return nil, 0, fmt.Errorf("decompress shared %s snapshot: %w", key, err)
		}
		if err := json.Unmarshal(payload, &cached); err != nil {
			return nil, 0, fmt.Errorf("decode shared %s snapshot: %w", key, err)
		}
		setCache(&cached)
		nextDelay := snapshot.RemainingFreshness()
		log.Debugf(
			"Statistics %s shared snapshot is still fresh; collection skipped, retrying at expiry in %s",
			key,
			nextDelay,
		)
		return &cached, nextDelay, nil
	}

	// Keep the complete collection on this connection so MySQL temporary
	// tables remain visible. Do not wrap this long-running analytical workload
	// in a transaction: a repeatable-read snapshot would retain undo history
	// while DataExplorer keeps writing.
	collected, err := collector(database)
	if err != nil {
		return nil, 0, err
	}
	payload, err := json.Marshal(collected)
	if err != nil {
		return nil, 0, err
	}
	sharedPayload, err := encodeSharedPayload(payload)
	if err != nil {
		return nil, 0, err
	}
	if err := models.UpsertStatisticsSnapshot(database, key, sharedPayload); err != nil {
		return nil, 0, err
	}
	log.Infof(
		"Statistics %s shared snapshot saved (json=%d bytes, gzip=%d bytes)",
		key,
		len(payload),
		len(sharedPayload),
	)
	if err := writeSnapshotFileAtomic(key, payload); err != nil {
		// MySQL is the shared source of truth. A failed local backup must not
		// discard a successfully committed shared snapshot.
		log.Warnf("Failed to write local %s statistics backup: %v", key, err)
	}
	setCache(collected)
	return collected, config.Period, nil
}

func loadSnapshot[T any](key string) (*T, error) {
	if StatisticsDB != nil {
		snapshot, err := models.GetStatisticsSnapshot(StatisticsDB, key, time.Second)
		if err == nil && snapshot != nil {
			var cached T
			payload, decodeErr := decodeSharedPayload(snapshot.Payload)
			if decodeErr != nil {
				return nil, fmt.Errorf("decompress shared %s snapshot: %w", key, decodeErr)
			}
			if err := json.Unmarshal(payload, &cached); err != nil {
				return nil, fmt.Errorf("decode shared %s snapshot: %w", key, err)
			}
			return &cached, nil
		}
		if err != nil {
			log.Warnf("Failed to load shared %s statistics snapshot; trying local backup: %v", key, err)
		}
	}

	payload, err := os.ReadFile(keyPath(key))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cached T
	if err := json.Unmarshal(payload, &cached); err != nil {
		return nil, err
	}
	return &cached, nil
}

func saveCurrentSnapshot[T any](key string, value *T) error {
	if value == nil {
		return errors.New("statistics data is nil")
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if StatisticsDB != nil {
		sharedPayload, err := encodeSharedPayload(payload)
		if err != nil {
			return err
		}
		if err := models.UpsertStatisticsSnapshot(StatisticsDB, key, sharedPayload); err != nil {
			return err
		}
	}
	return writeSnapshotFileAtomic(key, payload)
}

func encodeSharedPayload(payload []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(payload); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func decodeSharedPayload(payload []byte) ([]byte, error) {
	// Backward compatibility for snapshots written before compression support.
	if len(payload) < 2 || payload[0] != 0x1f || payload[1] != 0x8b {
		return payload, nil
	}
	reader, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func writeSnapshotFileAtomic(key string, payload []byte) error {
	directory := path.Join(util.GetProjectRootDirectory(), StatisticsDataPath)
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	target := keyPath(key)
	temp, err := os.CreateTemp(directory, "."+key+".*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if err := temp.Chmod(0644); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(payload); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, target); err != nil {
		// Windows does not replace an existing target with Rename. Production
		// Linux remains fully atomic; this fallback keeps local development usable.
		if removeErr := os.Remove(target); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return err
		}
		if retryErr := os.Rename(tempName, target); retryErr != nil {
			return retryErr
		}
	}
	return nil
}

func keyPath(key string) string {
	rootDir := util.GetProjectRootDirectory()
	return path.Join(rootDir, StatisticsDataPath, key+".json")
}

func logLoadedSnapshot(key string, value any) {
	if value == nil {
		log.Infof("Statistics %s cache is empty", key)
		return
	}
	log.Infof("Statistics %s cache loaded", key)
}
