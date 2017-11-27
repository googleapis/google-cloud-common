(function() {
  'use strict';

  angular
    .module('gcloud')
    .directive('moduleSwitcher', moduleSwitcher);

  /** @ngInject */
  function moduleSwitcher($state, manifest, util) {
    return {
      restrict: 'A',
      templateUrl: 'app/components/module-switcher/module-switcher.html',
      link: function(scope) {
        scope.modules = manifest.modules;

        scope.getUrl = function(module) {
          if (module.redirectTo) {
            return module.redirectTo;
          }

          return $state.href('docs.service', {
            module: module.id,
            version: module.versions[0],
            serviceId: module.defaultService
          });
        };

        scope.$watch($state.params, function() {
          var module = util.findWhere(manifest.modules, {
            id: $state.params.module
          });

          scope.selected = module.name;
        });
      }
    };
  }
}());
