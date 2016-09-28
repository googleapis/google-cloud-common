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
      repo: 'google-cloud-java'
    }, {
      friendly: 'Go',
      key: 'go',
      repo: 'google-cloud-go'
    }, {
      friendly: 'Node.js',
      key: 'node',
      repo: 'google-cloud-node'
    }, {
      friendly: 'PHP',
      key: 'php',
      repo: 'google-cloud-php'
    }, {
      friendly: 'Python',
      key: 'python',
      repo: 'gcloud-python'
    }, {
      friendly: 'Ruby',
      key: 'ruby',
      repo: 'google-cloud-ruby'
    }]);

}());
