package models

import (
	"team.gg-server/libs/db"
	"time"
)

type CustomGameConfigurationDAO struct {
	Id               string    `db:"id" json:"id"`
	Name             string    `db:"name" json:"name"`
	CreatorUid       string    `db:"creator_uid" json:"creatorUid"`
	CreatedAt        time.Time `db:"created_at" json:"createdAt"`
	LastUpdatedAt    time.Time `db:"last_updated_at" json:"lastUpdatedAt"`
	IsPublic         bool      `db:"is_public" json:"isPublic"`
	Fairness         float64   `db:"fairness" json:"fairness"`
	LineFairness     float64   `db:"line_fairness" json:"lineFairness"`
	TierFairness     float64   `db:"tier_fairness" json:"tierFairness"`
	LineSatisfaction float64   `db:"line_satisfaction" json:"lineSatisfaction"`

	LineFairnessWeight     float64 `db:"line_fairness_weight" json:"lineFairnessWeight"`
	TierFairnessWeight     float64 `db:"tier_fairness_weight" json:"tierFairnessWeight"`
	LineSatisfactionWeight float64 `db:"line_satisfaction_weight" json:"lineSatisfactionWeight"`

	TopInfluenceWeight     float64 `db:"top_influence_weight" json:"topInfluenceWeight"`
	JungleInfluenceWeight  float64 `db:"jungle_influence_weight" json:"jungleInfluenceWeight"`
	MidInfluenceWeight     float64 `db:"mid_influence_weight" json:"midInfluenceWeight"`
	AdcInfluenceWeight     float64 `db:"adc_influence_weight" json:"adcInfluenceWeight"`
	SupportInfluenceWeight float64 `db:"support_influence_weight" json:"supportInfluenceWeight"`
}

func (c *CustomGameConfigurationDAO) Upsert(db db.Context) error {
	if _, err := db.Exec(`
	INSERT INTO custom_game_configurations (
		id, name, creator_uid, created_at, last_updated_at, is_public, fairness, line_fairness, tier_fairness, line_satisfaction,
		line_fairness_weight, tier_fairness_weight, line_satisfaction_weight,
		top_influence_weight, jungle_influence_weight, mid_influence_weight, adc_influence_weight, support_influence_weight
	) VALUES (
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
	) ON DUPLICATE KEY UPDATE
	    name = ?,
		last_updated_at = ?,
		is_public = ?,
		fairness = ?,
		line_fairness = ?,
		tier_fairness = ?,
		line_satisfaction = ?,
		line_fairness_weight = ?,
		tier_fairness_weight = ?,
		line_satisfaction_weight = ?,
		top_influence_weight = ?,
		jungle_influence_weight = ?,
		mid_influence_weight = ?,
		adc_influence_weight = ?,
		support_influence_weight = ?`,
		c.Id, c.Name, c.CreatorUid, c.CreatedAt, c.LastUpdatedAt, c.IsPublic, c.Fairness, c.LineFairness, c.TierFairness, c.LineSatisfaction,
		c.LineFairnessWeight, c.TierFairnessWeight, c.LineSatisfactionWeight,
		c.TopInfluenceWeight, c.JungleInfluenceWeight, c.MidInfluenceWeight, c.AdcInfluenceWeight, c.SupportInfluenceWeight,
		c.Name, c.LastUpdatedAt, c.IsPublic, c.Fairness, c.LineFairness, c.TierFairness, c.LineSatisfaction,
		c.LineFairnessWeight, c.TierFairnessWeight, c.LineSatisfactionWeight,
		c.TopInfluenceWeight, c.JungleInfluenceWeight, c.MidInfluenceWeight, c.AdcInfluenceWeight, c.SupportInfluenceWeight,
	); err != nil {
		return err
	}
	return nil
}

func GetCustomGameDAOs_byCreatorUid(db db.Context, uid string) ([]CustomGameConfigurationDAO, error) {
	var customGameDAOs []CustomGameConfigurationDAO
	if err := db.Select(&customGameDAOs, `
		SELECT * FROM custom_game_configurations WHERE creator_uid = ?
	`, uid); err != nil {
		return nil, err
	}
	return customGameDAOs, nil
}

func GetCustomGameDAO_byId(db db.Context, id string) (*CustomGameConfigurationDAO, bool, error) {
	var customGameDAO CustomGameConfigurationDAO
	if err := db.Get(&customGameDAO, `
		SELECT * FROM custom_game_configurations WHERE id = ?
	`, id); err != nil {
		return nil, false, err
	}
	return &customGameDAO, true, nil
}

func UpdateCustomGameConfigurationName(db db.Context, id, name string, updatedAt time.Time) error {
	_, err := db.Exec(`
		UPDATE custom_game_configurations
		SET name = ?, last_updated_at = ?
		WHERE id = ?
	`, name, updatedAt, id)
	return err
}
