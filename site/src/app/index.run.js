(function() {
  'use strict';

  angular
    .module('gcloud')
    .run(runBlock);

  /** @ngInject */
  function runBlock($state, $location, $rootScope, $timeout, $document, manifest) {
    if (!manifest.moduleName) {
      manifest.moduleName = 'google-cloud-' + manifest.lang;
    }
    $document.find('title').text(manifest.moduleName);

    angular.extend($rootScope, manifest);

    $rootScope.$on('$stateChangeError', function() {
      // // uncomment for debugging
      // // console.log(arguments);

      var versionIndex = manifest.modules ? 3 : 2;
      var defaultModule = manifest.defaultModule || '';

      var path = $location.path();
      var params = path.split('/');
      var version = params[versionIndex];

      if (version && version.indexOf('v') === 0) {
        params[versionIndex] = version.replace('v', '');
      } else if (!params[params.length -1]) {
        params.pop();
      } else {
        params = ['docs', defaultModule, 'latest', 'not-found'];
      }

      $timeout(function() {
        $location.path(params.join('/'));
      });
    });
  }

}());
