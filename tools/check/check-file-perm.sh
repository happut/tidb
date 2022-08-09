#!/usr/bin/env sh
# Copyright 2022 PingCAP, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

find . \( -name "*.png" \
	-o -name "*.md" \
	-o -name "*.toml" \
	-o -name "*.yaml" \
	-o -name "*.json" \
	-o -name "*.bazel" \
	-o -name "*.txt" \
	-o -name "*.jpg" \
	-o -name "*.png" \
	-o -name "go.mod" \
	-o -name "go.sum" \
\) -exec chmod -x {} \;

# no files should be changed
if [ -z "$(git status --untracked-files=no --porcelain)" ]; then 
	exit 0
else 
	echo "you may have files with wrong permission, for example an executable txt or png file"
	git status
	exit 1
fi

