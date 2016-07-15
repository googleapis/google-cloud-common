/* global hljs: true */
(function() {
  'use strict';

  angular
    .module('gcloud')
    .constant('hljs', hljs)
    .constant('langs', [{
      friendly: '.NET',
      key: 'dotnet',
      repo: 'google-cloud-dotnet'
    }, {
      friendly: 'Java',
      key: 'java',
      repo: 'gcloud-java'
    }, {
      friendly: 'Node.js',
      key: 'node',
      repo: 'gcloud-node'
    }, {
      friendly: 'PHP',
      key: 'php',
      repo: 'gcloud-php'
    }, {
      friendly: 'Python',
      key: 'python',
      repo: 'gcloud-python'
    }, {
      friendly: 'Ruby',
      key: 'ruby',
      repo: 'gcloud-ruby'
    }]);

}());
