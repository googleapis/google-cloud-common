#/bin/bash

# Copyright 2015 Google Inc. All rights reserved.
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

set -ev

# This script takes the doc contents that exist under the `src` directory and
# pushes them to a the `src` directory in the gh-pages branch of the target
# repos (gcloud-node, gcloud-php, etc...)
cd site
npm install
gulp lint


# Only execute this script if this is a push to master.
if [ "${TRAVIS_BRANCH}" != "master" ] || [ "${TRAVIS_PULL_REQUEST}" != "false" ]
  then exit 0
fi

# Install dependencies & run the build (minify, concatenate dependencies, etc.)
bower install
gulp build

function deploy_docs {
  # Pull down the target client library's gh-pages branch.
  git submodule add -q -f --name $1 -b gh-pages https://${GH_OAUTH_TOKEN}@github.com/$1 gh-pages
  cd gh-pages

  # Remove the old site content. We're going to replace it.
  if [ -d src ]
    then git rm -rf src
  fi

  # `gulp build` earlier built the `dist` directory:
  #   ../
  #     dist/
  #       index.html
  #       src/
  #         *.js
  #         *.css
  cp -R ../dist/* .

  # Commit and push using the encrypted GitHub oauth token.
  # Allow "git add" to fail if there aren't new files.
  set +e
  git add .
  set -e
  if [[ -n "$(git status --porcelain)" ]]; then
    git config user.name "travis-ci"
    git config user.email "travis@travis-ci.org"
    git commit -m "Pushing a doc site update

    https://github.com/GoogleCloudPlatform/gcloud-common/commit/${TRAVIS_COMMIT}"
    git status
    git push -q https://${GH_OAUTH_TOKEN}@github.com/$1 HEAD:gh-pages
  else
    echo "Nothing to commit. Exiting without pushing changes."
  fi
  
  # Remove the gh-pages submodule.
  cd ..
  git submodule deinit -q -f gh-pages
  git rm -f gh-pages ../.gitmodules
  rm -rf ../.git/modules/gh-pages
}

deploy_docs "googlecloudplatform/google-cloud-node"
deploy_docs "googlecloudplatform/gcloud-python"
deploy_docs "googlecloudplatform/google-cloud-ruby"
