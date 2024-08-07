/*
NaiveSystems Analyze - A tool for static code analysis
Copyright (C) 2023  Naive Systems Ltd.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

/*
This package should not import any packages of other analyzers to
avoid recursive import.
*/
package filter

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	pb "naive.systems/analyzer/analyzer/proto"
	"naive.systems/analyzer/misra/checker_integration/checkrule"
	"naive.systems/analyzer/misra/checker_integration/compilecommand"
)

var kCppSuffixs = []string{"cpp", "cc", "cxx", "c++", "hpp"}
var KSupportImplementationSuffixs = []string{"c", "cpp", "cc", "cxx", "c++"}

func IsCCFile(path string) bool {
	for _, suffix := range KSupportImplementationSuffixs {
		if strings.HasSuffix(path, "."+suffix) {
			return true
		}
	}
	return false
}

func DeleteExceedResults(allResults *pb.ResultsList, checkRules []checkrule.CheckRule) *pb.ResultsList {
	maxReportNumMap := make(map[string]int)
	for _, checkRule := range checkRules {
		if checkRule.JSONOptions.MaxReportNum != nil {
			maxReportNumMap[checkRule.Name] = *checkRule.JSONOptions.MaxReportNum
		}
	}
	errHashMap := make(map[string]int)
	rtnResults := make([]*pb.Result, 0)
	for _, currentResult := range allResults.Results {
		if currentResult.Ruleset == "" || currentResult.RuleId == "" {
			glog.Errorf("unknown rule: %s/%s", currentResult.Ruleset, currentResult.RuleId)
			rtnResults = append(rtnResults, currentResult)
			continue
		}
		rule := fmt.Sprintf("%s/%s", currentResult.Ruleset, currentResult.RuleId)
		if _, exist := maxReportNumMap[rule]; !exist {
			rtnResults = append(rtnResults, currentResult)
			continue
		}
		if _, exist := errHashMap[rule]; !exist {
			errHashMap[rule] = 0
			rtnResults = append(rtnResults, currentResult)
			continue
		}
		errHashMap[rule]++
		if errHashMap[rule] < maxReportNumMap[rule] {
			rtnResults = append(rtnResults, currentResult)
		}
	}
	allResults.Results = rtnResults
	return allResults
}

func DeleteResultsWithCertainSuffixs(allResults *pb.ResultsList, suffix []string) *pb.ResultsList {
	rtnResults := make([]*pb.Result, 0)
	suffixs := make(map[string]struct{})

	for _, str := range suffix {
		suffixs[str] = struct{}{}
	}

	for _, currentResult := range allResults.Results {
		if _, ok := suffixs[filepath.Ext(currentResult.Path)]; !ok {
			rtnResults = append(rtnResults, currentResult)
		}
	}
	allResults.Results = rtnResults
	return allResults
}

func DeleteCppResults(allResults *pb.ResultsList) *pb.ResultsList {
	return DeleteResultsWithCertainSuffixs(allResults, kCppSuffixs)
}

func DeleteCompileCommandsFromCCJson(compileCommandsPath string, percent float64) error {
	if percent <= 0 || percent >= 1 {
		return nil
	}

	compileCommands, err := compilecommand.ReadCompileCommandsFromFile(compileCommandsPath)
	if err != nil {
		glog.Errorf("compilecommand.ReadCompileCommandsFromFiles: %v", err)
		return err
	}
	fileSet := map[string]bool{}

	for _, command := range *compileCommands {
		if _, exist := fileSet[command.File]; !exist {
			// The value is useless, just filter duplicated commands
			fileSet[command.File] = true
		}
	}
	total := len(fileSet)
	keep := int(math.Ceil(float64(total) * percent))
	keepFiles := []string{}
	for key := range fileSet {
		if len(keepFiles) < keep {
			keepFiles = append(keepFiles, key)
			continue
		}
		index := rand.Intn(keep)
		if index < keep {
			keepFiles[index] = key
		}
	}

	keepFileSet := map[string]bool{}
	for _, file := range keepFiles {
		keepFileSet[file] = true
	}

	filteredCompileCommands := []compilecommand.CompileCommand{}
	for _, command := range *compileCommands {
		if _, exist := keepFileSet[command.File]; exist {
			filteredCompileCommands = append(filteredCompileCommands, command)
		}
	}

	content, err := json.Marshal(filteredCompileCommands)
	if err != nil {
		glog.Errorf("createFakeCCJson: Failed to marshal command list %v", err)
		return err
	}
	err = os.WriteFile(compileCommandsPath, content, os.ModePerm)
	if err != nil {
		glog.Errorf("createFakeCCJson: Failed to write ccjson file %v", err)
		return err
	}
	return nil
}
