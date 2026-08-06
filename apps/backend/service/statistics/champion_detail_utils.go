package statistics

import (
	"fmt"
	log "github.com/shyunku-libraries/go-logger"
	"sort"
	"strconv"
	"team.gg-server/service"
	"team.gg-server/types"
)

func GetLowerDepthItems(itemId int) ([]int, error) {
	item, exists := service.Items[itemId]
	if !exists {
		return nil, fmt.Errorf("item not found: %d", itemId)
	}
	lowerItems := make([]int, 0)
	if item.Depth == nil || *item.Depth <= 1 {
		return lowerItems, nil
	}
	if item.From == nil || len(*item.From) == 0 {
		return lowerItems, nil
	}
	for _, fromIdStr := range *item.From {
		fromId, err := strconv.Atoi(fromIdStr)
		if err != nil {
			return nil, err
		}
		lowerItems = append(lowerItems, fromId)
	}
	return lowerItems, nil
}

func GetDepth1Items(itemId int) ([]int, error) {
	item, exists := service.Items[itemId]
	if !exists {
		return nil, fmt.Errorf("item not found: %d", itemId)
	}
	selfItems := []int{itemId}
	if item.Depth == nil || *item.Depth <= 1 {
		return selfItems, nil
	}
	if item.From == nil || len(*item.From) == 0 {
		return selfItems, nil
	}
	subItems, err := GetLowerDepthItems(itemId)
	if err != nil {
		return nil, err
	}
	lowerDepthItems := make([]int, 0)
	for _, subItem := range subItems {
		depth1Items, err := GetDepth1Items(subItem)
		if err != nil {
			return nil, err
		}
		lowerDepthItems = append(lowerDepthItems, depth1Items...)
	}
	lowerItemSet := make(map[int]bool)
	for _, lowerItem := range lowerDepthItems {
		lowerItemSet[lowerItem] = true
	}
	uniqueLowerDepthItems := make([]int, 0)
	for lowerItem := range lowerItemSet {
		uniqueLowerDepthItems = append(uniqueLowerDepthItems, lowerItem)
	}
	return uniqueLowerDepthItems, nil
}

type positionItemCount struct {
	itemId int
	count  int
}

func getDescSortedPositionItemCounts(positionItems []int) []positionItemCount {
	positionItemCountMap := make(map[int]int)
	for _, item := range positionItems {
		if _, exists := positionItemCountMap[item]; !exists {
			positionItemCountMap[item] = 0
		}
		positionItemCountMap[item]++
	}
	positionItemCounts := make([]positionItemCount, 0)
	for itemId, count := range positionItemCountMap {
		positionItemCounts = append(positionItemCounts, positionItemCount{itemId: itemId, count: count})
	}
	sort.SliceStable(positionItemCounts, func(i, j int) bool {
		return positionItemCounts[i].count > positionItemCounts[j].count
	})
	return positionItemCounts
}

func getPositionItemTags(positionItems []int) map[string]bool {
	positionItemTags := make(map[string]bool)
	for _, itemId := range positionItems {
		item, exists := service.Items[itemId]
		if !exists {
			log.Errorf("item not found: %d", itemId)
			continue
		}
		for _, tag := range item.Tags {
			positionItemTags[tag] = true
		}
	}
	return positionItemTags
}

func getValidItems(itemIdList []*int) []int {
	validItems := make([]int, 0)
	for _, itemId := range itemIdList {
		if itemId == nil {
			continue
		}
		copied := *itemId
		if _, exists := service.Items[copied]; exists {
			validItems = append(validItems, copied)
		}
	}
	return validItems
}

func getLowDepthItemRecommendations(championId int, teamPosition string, positionItemTags map[string]bool) (map[int]service.ItemDataVO, error) {
	championIdStr := strconv.Itoa(championId)
	champion, exists := service.Champions[championIdStr]
	if !exists {
		return nil, fmt.Errorf("champion not found: %d", championId)
	}

	isAdType := champion.Info.Attack >= 5
	isApType := champion.Info.Magic >= 5

	positionLowDepthItemRecommendations := make(map[int]service.ItemDataVO)
	for itemId, item := range service.Items {
		if !(item.Depth == nil &&
			item.Gold.Purchasable &&
			item.Gold.Total > 0 &&
			item.RequiredAlly == nil &&
			item.AvailableOnMapId(types.MapTypeSummonersRift)) {
			continue
		}
		foundTags := 0
		tagMap := make(map[string]bool)
		for _, tag := range item.Tags {
			if _, exists := positionItemTags[tag]; exists {
				foundTags++
			}
			tagMap[tag] = true
		}

		hasTag := func(tag string) bool {
			_, exists := tagMap[tag]
			return exists
		}

		onlyForJungle, onlyForLane := hasTag(types.ItemTagJungle), hasTag(types.ItemTagLane)
		onlyForAd := hasTag(types.ItemTagDamage) || hasTag(types.ItemTagCriticalStrike)
		onlyForAp := hasTag(types.ItemTagSpellDamage) || hasTag(types.ItemTagSpellVamp)
		forVision, _ := hasTag(types.ItemTagVision), hasTag(types.ItemTagGoldPer)
		forConsumable := hasTag(types.ItemTagConsumable)

		if onlyForJungle && onlyForLane {
			onlyForJungle, onlyForLane = false, false
		}
		if onlyForAd && onlyForAp {
			onlyForAd, onlyForAp = false, false
		}

		except := false
		if (teamPosition == types.TeamPositionJungle && onlyForLane) || (teamPosition != types.TeamPositionJungle && onlyForJungle) {
			except = true
		}
		if teamPosition != types.TeamPositionSupport && forVision && !forConsumable {
			except = true
		}
		if isAdType != isApType {
			// AD/AP 구분이 확실한 경우에만 필터링
			if (isAdType && onlyForAp) || (isApType && onlyForAd) {
				except = true
			}
		}

		if !except {
			positionLowDepthItemRecommendations[itemId] = item
		}
	}

	return positionLowDepthItemRecommendations, nil
}

func getItemTrees(
	positionItemCounts []positionItemCount,
	lowDepthItemRecommends map[int]service.ItemDataVO,
	majorItems []int,
) ([]int, []int, []int, error) {
	// gold sorters
	descSorter := func(i, j int) bool {
		itemI, existsI := lowDepthItemRecommends[i]
		itemJ, existsJ := lowDepthItemRecommends[j]
		if !existsI || !existsJ {
			return false
		}
		return itemI.Gold.Total > itemJ.Gold.Total
	}
	ascSorter := func(i, j int) bool {
		itemI, existsI := lowDepthItemRecommends[i]
		itemJ, existsJ := lowDepthItemRecommends[j]
		if !existsI || !existsJ {
			return false
		}
		return itemI.Gold.Total < itemJ.Gold.Total
	}

	// collect start, basic items
	startItems := make([]int, 0)
	for itemId, item := range lowDepthItemRecommends {
		if item.Gold.Total < 500 && item.Into == nil {
			startItems = append(startItems, itemId)
		}
	}
	basicItems := make([]int, 0)
	basicItemMap := make(map[int]bool)
	for _, itemId := range majorItems {
		depth1Items, err := GetDepth1Items(itemId)
		if err != nil {
			log.Error(err)
			return nil, nil, nil, err
		}
		for _, depth1Item := range depth1Items {
			basicItemMap[depth1Item] = true
		}
	}
	for itemId, _ := range basicItemMap {
		basicItems = append(basicItems, itemId)
	}

	// collect sub items
	subItems := make([]int, 0)
	for _, itemCount := range positionItemCounts {
		foundInThisMeta := false
		for _, itemId := range majorItems {
			if itemId == itemCount.itemId {
				foundInThisMeta = true
				break
			}
		}
		if !foundInThisMeta {
			subItems = append(subItems, itemCount.itemId)
		}
	}
	sort.SliceStable(startItems, descSorter)
	sort.SliceStable(basicItems, ascSorter)
	sort.SliceStable(subItems, descSorter)

	return startItems, basicItems, subItems, nil
}

func getSlotsFromStyle(primaryStyleId, subStyleId int) ([]PerkSlot, []PerkSlot, []PerkSlot, error) {
	primaryPerkStyle, ok := service.PerkStyles[primaryStyleId]
	if !ok {
		return nil, nil, nil, fmt.Errorf("primary perk style not found: %d", primaryStyleId)
	}
	subPerkStyle, ok := service.PerkStyles[subStyleId]
	if !ok {
		return nil, nil, nil, fmt.Errorf("sub perk style not found: %d", subStyleId)
	}

	mainSlots := make([]PerkSlot, 0)
	subSlots := make([]PerkSlot, 0)
	statSlots := make([]PerkSlot, 0)
	for _, slot := range primaryPerkStyle.Slots {
		if slot.Type == types.PerkSlotTypeStatMod {
			statSlots = append(statSlots, slot)
		} else {
			mainSlots = append(mainSlots, slot)
		}
	}
	for _, slot := range subPerkStyle.Slots {
		if slot.Type != types.PerkSlotTypeStatMod && slot.Type != types.PerkSlotTypeKeystone {
			subSlots = append(subSlots, slot)
		}
	}

	return mainSlots, subSlots, statSlots, nil
}
