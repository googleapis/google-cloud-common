/* global hljs: true */
(function() {
  'use strict';

  angular
    .module('gcloud')
    .constant('hljs', hljs)
    .constant('langs', [{
      friendly: 'Java',
      key: 'java'
    }, {
      friendly: 'Node.js',
      key: 'node'
    }, {
      friendly: 'PHP',
      key: 'php'
    }, {
      friendly: 'Python',
      key: 'python'
    }, {
      friendly: 'Ruby',
      key: 'ruby'
    }]);

}());
