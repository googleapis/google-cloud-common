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
      friendly: 'C++',
      key: 'cpp',
      repo: 'google-cloud-cpp'
    }, {
      friendly: 'Go',
      key: 'go',
      repo: 'google-cloud-go'
    }, {
      friendly: 'Java',
      key: 'java',
      repo: 'google-cloud-java'
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
      repo: 'google-cloud-python'
    }, {
      friendly: 'Ruby',
      key: 'ruby',
      repo: 'google-cloud-ruby'
    }]);

}());
