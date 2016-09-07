(function() {
  'use strict';

  angular
    .module('gcloud')
    .run(runBlock);

  /** @ngInject */
  function runBlock($state, $location, $rootScope, $timeout, manifest) {
    if (!manifest.moduleName) {
      manifest.moduleName = 'gcloud-' + manifest.lang;
    }

    angular.extend($rootScope, manifest);

    $rootScope.$on('$stateChangeError', function() {
      // uncomment for debugging
      // console.log(arguments);

      var path = $location.path();
      var params = path.split('/');
      var version = params[3];

      if (version && version.indexOf('v') === 0) {
        var normalizedVersion = version.replace('v', '');
        var normalizedPath = path.replace(version, normalizedVersion);

        return $timeout(function() {
          $location.path(normalizedPath);
        });
      }

      $state.go('docs.notfound');
    });
  }

}());
