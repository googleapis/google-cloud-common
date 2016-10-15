(function() {
  'use strict';

  angular
    .module('gcloud')
    .controller('ServiceCtrl', ServiceCtrl);

  /** @ngInject */
  function ServiceCtrl($scope, $state, DeeplinkService, DocsService, serviceObject, manifest) {
    var service = this;

    angular.extend(service, DocsService.setAsTrusted(serviceObject));

    service.methods = serviceObject.methods
      .map(DocsService.setAsTrusted)
      .sort(sortMethods);

    service.libraryTitle = manifest.libraryTitle || 'Google Cloud';
    service.methodNames = service.methods.map(getName);
    service.showGettingStarted = false;

    $scope.$on('$viewContentLoaded', watchMethod);

    function getName(method) {
      return method.name;
    }

    function watchMethod() {
      return DeeplinkService.watch($scope, getAnchor);
    }

    function getAnchor() {
      return $state.params && ($state.params.method || $state.params.section);
    }

    function sortMethods(a, b) {
      if (a.type === 'constructor') {
        return -1;
      }

      if (b.type === 'constructor') {
        return 1;
      }

      return +(a.name > b.name) || +(a.name === b.name) - 1;
    }
  }
}());
