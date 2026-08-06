package service

import (
	"team.gg-server/core"
	"team.gg-server/types"
	"time"
)

var (
	GetSupportedPositions = [...]string{
		types.PositionTop,
		types.PositionJungle,
		types.PositionMid,
		types.PositionAdc,
		types.PositionSupport,
	}
	GetPossibleTeamPositions = func() []CustomGameTeamPositionVO {
		return []CustomGameTeamPositionVO{
			{Team: 1, Position: types.PositionTop},
			{Team: 1, Position: types.PositionJungle},
			{Team: 1, Position: types.PositionMid},
			{Team: 1, Position: types.PositionAdc},
			{Team: 1, Position: types.PositionSupport},
			{Team: 2, Position: types.PositionTop},
			{Team: 2, Position: types.PositionJungle},
			{Team: 2, Position: types.PositionMid},
			{Team: 2, Position: types.PositionAdc},
			{Team: 2, Position: types.PositionSupport},
		}
	}
	GetInitialMatchCount = func() int {
		if core.IsProduction {
			return types.LoadInitialMatchCount
		}
		return types.LoadInitialMatchCountDev
	}
	GetLoadMoreMatchCount = func() int {
		if core.IsProduction {
			return types.LoadMoreMatchCount
		}
		return types.LoadMoreMatchCountDev
	}
	GetDataExplorerLoopPeriod = func() time.Duration {
		if core.IsProduction {
			return types.DataExplorerLoopPeriod
		}
		return types.DataExplorerLoopPeriodDev
	}
)
