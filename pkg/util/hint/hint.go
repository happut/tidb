// Copyright 2023 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hint

import (
	"bytes"
	"fmt"
	"strings"

	mysql "github.com/pingcap/tidb/pkg/errno"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/model"
	"github.com/pingcap/tidb/pkg/sessionctx"
	"github.com/pingcap/tidb/pkg/util/dbterror"
)

// Hint flags listed here are used by PlanBuilder.subQueryHintFlags.
const (
	// TiDBMergeJoin is hint enforce merge join.
	TiDBMergeJoin = "tidb_smj"
	// HintSMJ is hint enforce merge join.
	HintSMJ = "merge_join"
	// HintNoMergeJoin is the hint to enforce the query not to use merge join.
	HintNoMergeJoin = "no_merge_join"

	// TiDBBroadCastJoin indicates applying broadcast join by force.
	TiDBBroadCastJoin = "tidb_bcj"
	// HintBCJ indicates applying broadcast join by force.
	HintBCJ = "broadcast_join"
	// HintShuffleJoin indicates applying shuffle join by force.
	HintShuffleJoin = "shuffle_join"

	// HintStraightJoin causes TiDB to join tables in the order in which they appear in the FROM clause.
	HintStraightJoin = "straight_join"
	// HintLeading specifies the set of tables to be used as the prefix in the execution plan.
	HintLeading = "leading"

	// TiDBIndexNestedLoopJoin is hint enforce index nested loop join.
	TiDBIndexNestedLoopJoin = "tidb_inlj"
	// HintINLJ is hint enforce index nested loop join.
	HintINLJ = "inl_join"
	// HintINLHJ is hint enforce index nested loop hash join.
	HintINLHJ = "inl_hash_join"
	// HintINLMJ is hint enforce index nested loop merge join.
	HintINLMJ = "inl_merge_join"
	// HintNoIndexJoin is the hint to enforce the query not to use index join.
	HintNoIndexJoin = "no_index_join"
	// HintNoIndexHashJoin is the hint to enforce the query not to use index hash join.
	HintNoIndexHashJoin = "no_index_hash_join"
	// HintNoIndexMergeJoin is the hint to enforce the query not to use index merge join.
	HintNoIndexMergeJoin = "no_index_merge_join"
	// TiDBHashJoin is hint enforce hash join.
	TiDBHashJoin = "tidb_hj"
	// HintNoHashJoin is the hint to enforce the query not to use hash join.
	HintNoHashJoin = "no_hash_join"
	// HintHJ is hint enforce hash join.
	HintHJ = "hash_join"
	// HintHashJoinBuild is hint enforce hash join's build side
	HintHashJoinBuild = "hash_join_build"
	// HintHashJoinProbe is hint enforce hash join's probe side
	HintHashJoinProbe = "hash_join_probe"
	// HintHashAgg is hint enforce hash aggregation.
	HintHashAgg = "hash_agg"
	// HintStreamAgg is hint enforce stream aggregation.
	HintStreamAgg = "stream_agg"
	// HintMPP1PhaseAgg enforces the optimizer to use the mpp-1phase aggregation.
	HintMPP1PhaseAgg = "mpp_1phase_agg"
	// HintMPP2PhaseAgg enforces the optimizer to use the mpp-2phase aggregation.
	HintMPP2PhaseAgg = "mpp_2phase_agg"
	// HintUseIndex is hint enforce using some indexes.
	HintUseIndex = "use_index"
	// HintIgnoreIndex is hint enforce ignoring some indexes.
	HintIgnoreIndex = "ignore_index"
	// HintForceIndex make optimizer to use this index even if it thinks a table scan is more efficient.
	HintForceIndex = "force_index"
	// HintOrderIndex is hint enforce using some indexes and keep the index's order.
	HintOrderIndex = "order_index"
	// HintNoOrderIndex is hint enforce using some indexes and not keep the index's order.
	HintNoOrderIndex = "no_order_index"
	// HintAggToCop is hint enforce pushing aggregation to coprocessor.
	HintAggToCop = "agg_to_cop"
	// HintReadFromStorage is hint enforce some tables read from specific type of storage.
	HintReadFromStorage = "read_from_storage"
	// HintTiFlash is a label represents the tiflash storage type.
	HintTiFlash = "tiflash"
	// HintTiKV is a label represents the tikv storage type.
	HintTiKV = "tikv"
	// HintIndexMerge is a hint to enforce using some indexes at the same time.
	HintIndexMerge = "use_index_merge"
	// HintTimeRange is a hint to specify the time range for metrics summary tables
	HintTimeRange = "time_range"
	// HintIgnorePlanCache is a hint to enforce ignoring plan cache
	HintIgnorePlanCache = "ignore_plan_cache"
	// HintLimitToCop is a hint enforce pushing limit or topn to coprocessor.
	HintLimitToCop = "limit_to_cop"
	// HintMerge is a hint which can switch turning inline for the CTE.
	HintMerge = "merge"
	// HintSemiJoinRewrite is a hint to force we rewrite the semi join operator as much as possible.
	HintSemiJoinRewrite = "semi_join_rewrite"
	// HintNoDecorrelate indicates a LogicalApply not to be decorrelated.
	HintNoDecorrelate = "no_decorrelate"

	// HintMemoryQuota sets the memory limit for a query
	HintMemoryQuota = "memory_quota"
	// HintUseToja is a hint to optimize `in (select ...)` subquery into `join`
	HintUseToja = "use_toja"
	// HintNoIndexMerge is a hint to disable index merge
	HintNoIndexMerge = "no_index_merge"
	// HintMaxExecutionTime specifies the max allowed execution time in milliseconds
	HintMaxExecutionTime = "max_execution_time"

	// HintFlagSemiJoinRewrite corresponds to HintSemiJoinRewrite.
	HintFlagSemiJoinRewrite uint64 = 1 << iota
	// HintFlagNoDecorrelate corresponds to HintNoDecorrelate.
	HintFlagNoDecorrelate
)

const (
	// PreferINLJ indicates that the optimizer prefers to use index nested loop join.
	PreferINLJ uint = 1 << iota
	// PreferINLHJ indicates that the optimizer prefers to use index nested loop hash join.
	PreferINLHJ
	// PreferINLMJ indicates that the optimizer prefers to use index nested loop merge join.
	PreferINLMJ
	// PreferHJBuild indicates that the optimizer prefers to use hash join.
	PreferHJBuild
	// PreferHJProbe indicates that the optimizer prefers to use hash join.
	PreferHJProbe
	// PreferHashJoin indicates that the optimizer prefers to use hash join.
	PreferHashJoin
	// PreferNoHashJoin indicates that the optimizer prefers not to use hash join.
	PreferNoHashJoin
	// PreferMergeJoin indicates that the optimizer prefers to use merge join.
	PreferMergeJoin
	// PreferNoMergeJoin indicates that the optimizer prefers not to use merge join.
	PreferNoMergeJoin
	// PreferNoIndexJoin indicates that the optimizer prefers not to use index join.
	PreferNoIndexJoin
	// PreferNoIndexHashJoin indicates that the optimizer prefers not to use index hash join.
	PreferNoIndexHashJoin
	// PreferNoIndexMergeJoin indicates that the optimizer prefers not to use index merge join.
	PreferNoIndexMergeJoin
	// PreferBCJoin indicates that the optimizer prefers to use broadcast join.
	PreferBCJoin
	// PreferShuffleJoin indicates that the optimizer prefers to use shuffle join.
	PreferShuffleJoin
	// PreferRewriteSemiJoin indicates that the optimizer prefers to rewrite semi join.
	PreferRewriteSemiJoin

	// PreferLeftAsINLJInner indicates that the optimizer prefers to use left child as inner child of index nested loop join.
	PreferLeftAsINLJInner
	// PreferRightAsINLJInner indicates that the optimizer prefers to use right child as inner child of index nested loop join.
	PreferRightAsINLJInner
	// PreferLeftAsINLHJInner indicates that the optimizer prefers to use left child as inner child of index nested loop hash join.
	PreferLeftAsINLHJInner
	// PreferRightAsINLHJInner indicates that the optimizer prefers to use right child as inner child of index nested loop hash join.
	PreferRightAsINLHJInner
	// PreferLeftAsINLMJInner indicates that the optimizer prefers to use left child as inner child of index nested loop merge join.
	PreferLeftAsINLMJInner
	// PreferRightAsINLMJInner indicates that the optimizer prefers to use right child as inner child of index nested loop merge join.
	PreferRightAsINLMJInner
	// PreferLeftAsHJBuild indicates that the optimizer prefers to use left child as build child of hash join.
	PreferLeftAsHJBuild
	// PreferRightAsHJBuild indicates that the optimizer prefers to use right child as build child of hash join.
	PreferRightAsHJBuild
	// PreferLeftAsHJProbe indicates that the optimizer prefers to use left child as probe child of hash join.
	PreferLeftAsHJProbe
	// PreferRightAsHJProbe indicates that the optimizer prefers to use right child as probe child of hash join.
	PreferRightAsHJProbe

	// PreferHashAgg indicates that the optimizer prefers to use hash aggregation.
	PreferHashAgg
	// PreferStreamAgg indicates that the optimizer prefers to use stream aggregation.
	PreferStreamAgg
	// PreferMPP1PhaseAgg indicates that the optimizer prefers to use 1-phase aggregation.
	PreferMPP1PhaseAgg
	// PreferMPP2PhaseAgg indicates that the optimizer prefers to use 2-phase aggregation.
	PreferMPP2PhaseAgg
)

const (
	// PreferTiKV indicates that the optimizer prefers to use TiKV layer.
	PreferTiKV = 1 << iota
	// PreferTiFlash indicates that the optimizer prefers to use TiFlash layer.
	PreferTiFlash
)

// TODO: @qw4990 @hawkingrei, merge StmtHints with this structure.
//// StmtHints are hints that apply to the entire statement, like 'max_exec_time', 'memory_quota'.
//type StmtHints struct {
//}

// IndexJoinHints stores hint information about index nested loop join.
type IndexJoinHints struct {
	INLJTables  []HintedTable
	INLHJTables []HintedTable
	INLMJTables []HintedTable
}

// PlanHints are hints that are used to control the optimizer plan choices like 'use_index', 'hash_join'.
// TODO: move ignore_plan_cache, straight_join, no_decorrelate here.
type PlanHints struct {
	IndexJoin          IndexJoinHints // inlj_join, inlhj_join, inlmj_join
	NoIndexJoin        IndexJoinHints // no_inlj_join, no_inlhj_join, no_inlmj_join
	HashJoin           []HintedTable  // hash_join
	NoHashJoin         []HintedTable  // no_hash_join
	SortMergeJoin      []HintedTable  // merge_join
	NoMergeJoin        []HintedTable  // no_merge_join
	BroadcastJoin      []HintedTable  // bcj_join
	ShuffleJoin        []HintedTable  // shuffle_join
	IndexHintList      []HintedIndex  // use_index, ignore_index
	IndexMergeHintList []HintedIndex  // use_index_merge
	TiFlashTables      []HintedTable  // isolation_read_engines(xx=tiflash)
	TiKVTables         []HintedTable  // isolation_read_engines(xx=tikv)
	LeadingJoinOrder   []HintedTable  // leading
	HJBuild            []HintedTable  // hash_join_build
	HJProbe            []HintedTable  // hash_join_probe

	// Hints belows are not associated with any particular table.
	Agg              AggHints // hash_agg, merge_agg, agg_to_cop
	PreferLimitToCop bool     // limit_to_cop
	CTEMerge         bool     // merge
	TimeRangeHint    ast.HintTimeRange
}

// HintedTable indicates which table this hint should take effect on.
type HintedTable struct {
	DBName       model.CIStr   // the database name
	TblName      model.CIStr   // the table name
	Partitions   []model.CIStr // partition information
	SelectOffset int           // the select block offset of this hint
	Matched      bool          // whether this hint is applied successfully
}

// HintedIndex indicates which index this hint should take effect on.
type HintedIndex struct {
	DBName     model.CIStr    // the database name
	TblName    model.CIStr    // the table name
	Partitions []model.CIStr  // partition information
	IndexHint  *ast.IndexHint // the original parser index hint structure
	// Matched indicates whether this index hint
	// has been successfully applied to a DataSource.
	// If an HintedIndex is not Matched after building
	// a Select statement, we will generate a warning for it.
	Matched bool
}

// Match checks whether the hint is matched with the given dbName and tblName.
func (hint *HintedIndex) Match(dbName, tblName model.CIStr) bool {
	return hint.TblName.L == tblName.L &&
		(hint.DBName.L == dbName.L ||
			hint.DBName.L == "*") // for universal bindings, e.g. *.t
}

// HintTypeString returns the string representation of the hint type.
func (hint *HintedIndex) HintTypeString() string {
	switch hint.IndexHint.HintType {
	case ast.HintUse:
		return "use_index"
	case ast.HintIgnore:
		return "ignore_index"
	case ast.HintForce:
		return "force_index"
	}
	return ""
}

// IndexString formats the IndexHint as DBName.tableName[, indexNames].
func (hint *HintedIndex) IndexString() string {
	var indexListString string
	indexList := make([]string, len(hint.IndexHint.IndexNames))
	for i := range hint.IndexHint.IndexNames {
		indexList[i] = hint.IndexHint.IndexNames[i].L
	}
	if len(indexList) > 0 {
		indexListString = fmt.Sprintf(", %s", strings.Join(indexList, ", "))
	}
	return fmt.Sprintf("%s.%s%s", hint.DBName, hint.TblName, indexListString)
}

// AggHints stores Agg hint information.
type AggHints struct {
	PreferAggType  uint
	PreferAggToCop bool
}

// IfPreferMergeJoin checks whether the join hint is merge join.
func (pHints *PlanHints) IfPreferMergeJoin(tableNames ...*HintedTable) bool {
	return pHints.MatchTableName(tableNames, pHints.SortMergeJoin)
}

// IfPreferBroadcastJoin checks whether the join hint is broadcast join.
func (pHints *PlanHints) IfPreferBroadcastJoin(tableNames ...*HintedTable) bool {
	return pHints.MatchTableName(tableNames, pHints.BroadcastJoin)
}

// IfPreferShuffleJoin checks whether the join hint is shuffle join.
func (pHints *PlanHints) IfPreferShuffleJoin(tableNames ...*HintedTable) bool {
	return pHints.MatchTableName(tableNames, pHints.ShuffleJoin)
}

// IfPreferHashJoin checks whether the join hint is hash join.
func (pHints *PlanHints) IfPreferHashJoin(tableNames ...*HintedTable) bool {
	return pHints.MatchTableName(tableNames, pHints.HashJoin)
}

// IfPreferNoHashJoin checks whether the join hint is no hash join.
func (pHints *PlanHints) IfPreferNoHashJoin(tableNames ...*HintedTable) bool {
	return pHints.MatchTableName(tableNames, pHints.NoHashJoin)
}

// IfPreferNoMergeJoin checks whether the join hint is no merge join.
func (pHints *PlanHints) IfPreferNoMergeJoin(tableNames ...*HintedTable) bool {
	return pHints.MatchTableName(tableNames, pHints.NoMergeJoin)
}

// IfPreferHJBuild checks whether the join hint is hash join build side.
func (pHints *PlanHints) IfPreferHJBuild(tableNames ...*HintedTable) bool {
	return pHints.MatchTableName(tableNames, pHints.HJBuild)
}

// IfPreferHJProbe checks whether the join hint is hash join probe side.
func (pHints *PlanHints) IfPreferHJProbe(tableNames ...*HintedTable) bool {
	return pHints.MatchTableName(tableNames, pHints.HJProbe)
}

// IfPreferINLJ checks whether the join hint is index nested loop join.
func (pHints *PlanHints) IfPreferINLJ(tableNames ...*HintedTable) bool {
	return pHints.MatchTableName(tableNames, pHints.IndexJoin.INLJTables)
}

// IfPreferINLHJ checks whether the join hint is index nested loop hash join.
func (pHints *PlanHints) IfPreferINLHJ(tableNames ...*HintedTable) bool {
	return pHints.MatchTableName(tableNames, pHints.IndexJoin.INLHJTables)
}

// IfPreferINLMJ checks whether the join hint is index nested loop merge join.
func (pHints *PlanHints) IfPreferINLMJ(tableNames ...*HintedTable) bool {
	return pHints.MatchTableName(tableNames, pHints.IndexJoin.INLMJTables)
}

// IfPreferNoIndexJoin checks whether the join hint is no index join.
func (pHints *PlanHints) IfPreferNoIndexJoin(tableNames ...*HintedTable) bool {
	return pHints.MatchTableName(tableNames, pHints.NoIndexJoin.INLJTables)
}

// IfPreferNoIndexHashJoin checks whether the join hint is no index hash join.
func (pHints *PlanHints) IfPreferNoIndexHashJoin(tableNames ...*HintedTable) bool {
	return pHints.MatchTableName(tableNames, pHints.NoIndexJoin.INLHJTables)
}

// IfPreferNoIndexMergeJoin checks whether the join hint is no index merge join.
func (pHints *PlanHints) IfPreferNoIndexMergeJoin(tableNames ...*HintedTable) bool {
	return pHints.MatchTableName(tableNames, pHints.NoIndexJoin.INLMJTables)
}

// IfPreferTiFlash checks whether the hint hit the need of TiFlash.
func (pHints *PlanHints) IfPreferTiFlash(tableName *HintedTable) *HintedTable {
	return pHints.matchTiKVOrTiFlash(tableName, pHints.TiFlashTables)
}

// IfPreferTiKV checks whether the hint hit the need of TiKV.
func (pHints *PlanHints) IfPreferTiKV(tableName *HintedTable) *HintedTable {
	return pHints.matchTiKVOrTiFlash(tableName, pHints.TiKVTables)
}

func (*PlanHints) matchTiKVOrTiFlash(tableName *HintedTable, hintTables []HintedTable) *HintedTable {
	if tableName == nil {
		return nil
	}
	for i, tbl := range hintTables {
		if tableName.DBName.L == tbl.DBName.L && tableName.TblName.L == tbl.TblName.L && tbl.SelectOffset == tableName.SelectOffset {
			hintTables[i].Matched = true
			return &tbl
		}
	}
	return nil
}

// MatchTableName checks whether the hint hit the need.
// Only need either side matches one on the list.
// Even though you can put 2 tables on the list,
// it doesn't mean optimizer will reorder to make them
// join directly.
// Which it joins on with depend on sequence of traverse
// and without reorder, user might adjust themselves.
// This is similar to MySQL hints.
func (*PlanHints) MatchTableName(tables []*HintedTable, hintTables []HintedTable) bool {
	hintMatched := false
	for _, table := range tables {
		for i, curEntry := range hintTables {
			if table == nil {
				continue
			}
			if (curEntry.DBName.L == table.DBName.L || curEntry.DBName.L == "*") &&
				curEntry.TblName.L == table.TblName.L &&
				table.SelectOffset == curEntry.SelectOffset {
				hintTables[i].Matched = true
				hintMatched = true
				break
			}
		}
	}
	return hintMatched
}

// RemoveDuplicatedHints removes duplicated hints in this hit list.
func RemoveDuplicatedHints(hints []*ast.TableOptimizerHint) []*ast.TableOptimizerHint {
	if len(hints) < 2 {
		return hints
	}
	m := make(map[string]struct{}, len(hints))
	res := make([]*ast.TableOptimizerHint, 0, len(hints))
	for _, hint := range hints {
		key := RestoreTableOptimizerHint(hint)
		if _, ok := m[key]; ok {
			continue
		}
		m[key] = struct{}{}
		res = append(res, hint)
	}
	return res
}

// TableNames2HintTableInfo converts table names to HintedTable.
func TableNames2HintTableInfo(ctx sessionctx.Context, hintName string, hintTables []ast.HintTable, p *QBHintHandler, currentOffset int) []HintedTable {
	if len(hintTables) == 0 {
		return nil
	}
	hintTableInfos := make([]HintedTable, 0, len(hintTables))
	defaultDBName := model.NewCIStr(ctx.GetSessionVars().CurrentDB)
	isInapplicable := false
	for _, hintTable := range hintTables {
		tableInfo := HintedTable{
			DBName:       hintTable.DBName,
			TblName:      hintTable.TableName,
			Partitions:   hintTable.PartitionList,
			SelectOffset: p.GetHintOffset(hintTable.QBName, currentOffset),
		}
		if tableInfo.DBName.L == "" {
			tableInfo.DBName = defaultDBName
		}
		switch hintName {
		case TiDBMergeJoin, HintSMJ, TiDBIndexNestedLoopJoin, HintINLJ,
			HintINLHJ, HintINLMJ, TiDBHashJoin, HintHJ, HintLeading:
			if len(tableInfo.Partitions) > 0 {
				isInapplicable = true
			}
		}
		hintTableInfos = append(hintTableInfos, tableInfo)
	}
	if isInapplicable {
		ctx.GetSessionVars().StmtCtx.AppendWarning(
			fmt.Errorf("Optimizer Hint %s is inapplicable on specified partitions",
				Restore2JoinHint(hintName, hintTableInfos)))
		return nil
	}
	return hintTableInfos
}

func restore2TableHint(hintTables ...HintedTable) string {
	buffer := bytes.NewBufferString("")
	for i, table := range hintTables {
		buffer.WriteString(table.TblName.L)
		if len(table.Partitions) > 0 {
			buffer.WriteString(" PARTITION(")
			for j, partition := range table.Partitions {
				if j > 0 {
					buffer.WriteString(", ")
				}
				buffer.WriteString(partition.L)
			}
			buffer.WriteString(")")
		}
		if i < len(hintTables)-1 {
			buffer.WriteString(", ")
		}
	}
	return buffer.String()
}

// Restore2JoinHint restores join hint to string.
func Restore2JoinHint(hintType string, hintTables []HintedTable) string {
	if len(hintTables) == 0 {
		return strings.ToUpper(hintType)
	}
	buffer := bytes.NewBufferString("/*+ ")
	buffer.WriteString(strings.ToUpper(hintType))
	buffer.WriteString("(")
	buffer.WriteString(restore2TableHint(hintTables...))
	buffer.WriteString(") */")
	return buffer.String()
}

// Restore2IndexHint restores index hint to string.
func Restore2IndexHint(hintType string, hintIndex HintedIndex) string {
	buffer := bytes.NewBufferString("/*+ ")
	buffer.WriteString(strings.ToUpper(hintType))
	buffer.WriteString("(")
	buffer.WriteString(restore2TableHint(HintedTable{
		DBName:     hintIndex.DBName,
		TblName:    hintIndex.TblName,
		Partitions: hintIndex.Partitions,
	}))
	if hintIndex.IndexHint != nil && len(hintIndex.IndexHint.IndexNames) > 0 {
		for i, indexName := range hintIndex.IndexHint.IndexNames {
			if i > 0 {
				buffer.WriteString(",")
			}
			buffer.WriteString(" " + indexName.L)
		}
	}
	buffer.WriteString(") */")
	return buffer.String()
}

// Restore2StorageHint restores storage hint to string.
func Restore2StorageHint(tiflashTables, tikvTables []HintedTable) string {
	buffer := bytes.NewBufferString("/*+ ")
	buffer.WriteString(strings.ToUpper(HintReadFromStorage))
	buffer.WriteString("(")
	if len(tiflashTables) > 0 {
		buffer.WriteString("tiflash[")
		buffer.WriteString(restore2TableHint(tiflashTables...))
		buffer.WriteString("]")
		if len(tikvTables) > 0 {
			buffer.WriteString(", ")
		}
	}
	if len(tikvTables) > 0 {
		buffer.WriteString("tikv[")
		buffer.WriteString(restore2TableHint(tikvTables...))
		buffer.WriteString("]")
	}
	buffer.WriteString(") */")
	return buffer.String()
}

// ExtractUnmatchedTables extracts unmatched tables from hintTables.
func ExtractUnmatchedTables(hintTables []HintedTable) []string {
	var tableNames []string
	for _, table := range hintTables {
		if !table.Matched {
			tableNames = append(tableNames, table.TblName.O)
		}
	}
	return tableNames
}

var (
	errInternal = dbterror.ClassOptimizer.NewStd(mysql.ErrInternal)
)

// CollectUnmatchedHintWarnings collects warnings for unmatched hints from this TableHintInfo.
func CollectUnmatchedHintWarnings(hintInfo *PlanHints) (warnings []error) {
	warnings = append(warnings, collectUnmatchedIndexHintWarning(hintInfo.IndexHintList, false)...)
	warnings = append(warnings, collectUnmatchedIndexHintWarning(hintInfo.IndexMergeHintList, true)...)
	warnings = append(warnings, collectUnmatchedJoinHintWarning(HintINLJ, TiDBIndexNestedLoopJoin, hintInfo.IndexJoin.INLJTables)...)
	warnings = append(warnings, collectUnmatchedJoinHintWarning(HintINLHJ, "", hintInfo.IndexJoin.INLHJTables)...)
	warnings = append(warnings, collectUnmatchedJoinHintWarning(HintINLMJ, "", hintInfo.IndexJoin.INLMJTables)...)
	warnings = append(warnings, collectUnmatchedJoinHintWarning(HintSMJ, TiDBMergeJoin, hintInfo.SortMergeJoin)...)
	warnings = append(warnings, collectUnmatchedJoinHintWarning(HintBCJ, TiDBBroadCastJoin, hintInfo.BroadcastJoin)...)
	warnings = append(warnings, collectUnmatchedJoinHintWarning(HintShuffleJoin, HintShuffleJoin, hintInfo.ShuffleJoin)...)
	warnings = append(warnings, collectUnmatchedJoinHintWarning(HintHJ, TiDBHashJoin, hintInfo.HashJoin)...)
	warnings = append(warnings, collectUnmatchedJoinHintWarning(HintHashJoinBuild, "", hintInfo.HJBuild)...)
	warnings = append(warnings, collectUnmatchedJoinHintWarning(HintHashJoinProbe, "", hintInfo.HJProbe)...)
	warnings = append(warnings, collectUnmatchedJoinHintWarning(HintLeading, "", hintInfo.LeadingJoinOrder)...)
	warnings = append(warnings, collectUnmatchedStorageHintWarning(hintInfo.TiFlashTables, hintInfo.TiKVTables)...)
	return warnings
}

func collectUnmatchedIndexHintWarning(indexHints []HintedIndex, usedForIndexMerge bool) (warnings []error) {
	for _, hint := range indexHints {
		if !hint.Matched {
			var hintTypeString string
			if usedForIndexMerge {
				hintTypeString = "use_index_merge"
			} else {
				hintTypeString = hint.HintTypeString()
			}
			errMsg := fmt.Sprintf("%s(%s) is inapplicable, check whether the table(%s.%s) exists",
				hintTypeString,
				hint.IndexString(),
				hint.DBName,
				hint.TblName,
			)
			warnings = append(warnings, errInternal.FastGen(errMsg))
		}
	}
	return warnings
}

func collectUnmatchedJoinHintWarning(joinType string, joinTypeAlias string, hintTables []HintedTable) (warnings []error) {
	unMatchedTables := ExtractUnmatchedTables(hintTables)
	if len(unMatchedTables) == 0 {
		return
	}
	if len(joinTypeAlias) != 0 {
		joinTypeAlias = fmt.Sprintf(" or %s", Restore2JoinHint(joinTypeAlias, hintTables))
	}

	errMsg := fmt.Sprintf("There are no matching table names for (%s) in optimizer hint %s%s. Maybe you can use the table alias name",
		strings.Join(unMatchedTables, ", "), Restore2JoinHint(joinType, hintTables), joinTypeAlias)
	warnings = append(warnings, errInternal.GenWithStack(errMsg))
	return warnings
}

func collectUnmatchedStorageHintWarning(tiflashTables, tikvTables []HintedTable) (warnings []error) {
	unMatchedTiFlashTables := ExtractUnmatchedTables(tiflashTables)
	unMatchedTiKVTables := ExtractUnmatchedTables(tikvTables)
	if len(unMatchedTiFlashTables)+len(unMatchedTiKVTables) == 0 {
		return
	}
	errMsg := fmt.Sprintf("There are no matching table names for (%s) in optimizer hint %s. Maybe you can use the table alias name",
		strings.Join(append(unMatchedTiFlashTables, unMatchedTiKVTables...), ", "),
		Restore2StorageHint(tiflashTables, tikvTables))
	warnings = append(warnings, errInternal.GenWithStack(errMsg))
	return warnings
}
