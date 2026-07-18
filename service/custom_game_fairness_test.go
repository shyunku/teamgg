package service

import (
	"math"
	"testing"

	"team.gg-server/types"
)

func customGameParticipant(team int, position string, rating int64, favor int) CustomGameTeamParticipantVO {
	positionFavor := CustomGameCandidatePositionFavorVO{}
	switch position {
	case types.PositionTop:
		positionFavor.Top = favor
	case types.PositionJungle:
		positionFavor.Jungle = favor
	case types.PositionMid:
		positionFavor.Mid = favor
	case types.PositionAdc:
		positionFavor.Adc = favor
	case types.PositionSupport:
		positionFavor.Support = favor
	}

	return CustomGameTeamParticipantVO{
		CustomGameCandidateVO: CustomGameCandidateVO{
			CustomRank:    &SummonerRankVO{RatingPoint: rating},
			PositionFavor: positionFavor,
		},
		Team:     team,
		Position: position,
	}
}

func balancedParticipants(team1Rating, team2Rating int64, favor int) map[string]CustomGameTeamParticipantVO {
	participants := make(map[string]CustomGameTeamParticipantVO)
	for _, position := range GetSupportedPositions {
		participants[position+"-1"] = customGameParticipant(1, position, team1Rating, favor)
		participants[position+"-2"] = customGameParticipant(2, position, team2Rating, favor)
	}
	return participants
}

func assertClose(t *testing.T, name string, actual, expected float64) {
	t.Helper()
	if math.Abs(actual-expected) > 1e-9 {
		t.Fatalf("%s: got %.12f, want %.12f", name, actual, expected)
	}
}

func testBalanceWeights() CustomGameConfigurationWeightsVO {
	return CustomGameConfigurationWeightsVO{
		TierFairness:     0.4,
		LineFairness:     0.2,
		LineSatisfaction: 0.4,
	}
}

func TestCalculateCustomGameConfigFairnessPerfectBalance(t *testing.T) {
	balance, err := calculateCustomGameConfigFairness(balancedParticipants(1000, 1000, 2), testBalanceWeights())
	if err != nil {
		t.Fatal(err)
	}

	assertClose(t, "team fairness", balance.TierFairness, 1)
	assertClose(t, "line fairness", balance.LineFairness, 1)
	assertClose(t, "line satisfaction", balance.LineSatisfaction, 1)
	assertClose(t, "total fairness", balance.Fairness, 1)
}

func TestCalculateCustomGameConfigFairnessUsesFixedWeights(t *testing.T) {
	balance, err := calculateCustomGameConfigFairness(balancedParticipants(2000, 1000, 2), testBalanceWeights())
	if err != nil {
		t.Fatal(err)
	}

	assertClose(t, "team fairness", balance.TierFairness, 0.5)
	assertClose(t, "line fairness", balance.LineFairness, 0.5)
	assertClose(t, "line satisfaction", balance.LineSatisfaction, 1)
	assertClose(t, "total fairness", balance.Fairness, 0.7)
}

func TestCalculateCustomGameConfigFairnessNeutralPreference(t *testing.T) {
	balance, err := calculateCustomGameConfigFairness(balancedParticipants(1000, 1000, 0), testBalanceWeights())
	if err != nil {
		t.Fatal(err)
	}

	assertClose(t, "line satisfaction", balance.LineSatisfaction, 1.0/3.0)
	assertClose(t, "total fairness", balance.Fairness, 0.4+0.2+0.4/3.0)
}
