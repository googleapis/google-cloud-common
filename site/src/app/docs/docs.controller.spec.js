(function() {

  'use strict';

  describe('DocsCtrl', function() {
    var defaultDeps = {
      $state: {
        params: {}
      },
      langs: [],
      manifest: {
        versions: []
      },
      toc: {},
      types: [],
      lastBuiltDate: new Date(),
      versions: []
    };

    function createController(deps) {
      var $scope;

      inject(function($rootScope, $controller) {
        $scope = $rootScope.$new();
        var _deps = angular.extend({ $scope: $scope }, defaultDeps, deps);
        $controller('DocsCtrl as docs', _deps);
      });

      return $scope;
    }

    beforeEach(function() {
      module('gcloud.manifest', function($provide) {
        $provide.constant('manifest', defaultDeps.manifest);
      });
      module('gcloud');
    });

    describe('tagName', function() {
      it('should use the toc tagName field', function() {
        var fakeTagName = 'bigquery-0.1.0';
        var $scope = createController({
          toc: {
            tagName: fakeTagName
          }
        });

        expect($scope.docs.tagName).toBe(fakeTagName);
      });

      it('should use the state version param is tagName is omitted', function() {
        var fakeVersion = '0.41.0';
        var $scope = createController({
          $state: {
            params: {
              version: fakeVersion
            }
          }
        });

        expect($scope.docs.tagName).toBe(fakeVersion);
      });
    });
  });

}());
