(function() {
  'use strict';

  describe('anchor', function() {
    var FAKE_PATH = '/docs/logging/0.1.1/logging';
    var $scope;
    var $compile;

    beforeEach(function() {
      module('gcloud.manifest', function($provide) {
        $provide.constant('manifest', {
          versions: []
        });
      });
      module('gcloud');
    });

    beforeEach(inject(function($location, $rootScope, $injector) {
      $scope = $rootScope.$new();
      $compile = $injector.get('$compile');

      $location.path(FAKE_PATH);
    }));

    it('should recognize data attributes', function() {
      var fakeAnchor = 'test';
      var el = angular.element('<a data-anchor="' + fakeAnchor + '">Test</a>');

      $compile(el)($scope);
      $scope.$digest();

      var expectedPath = '#' + FAKE_PATH + '?section=' + fakeAnchor;

      expect(el.attr('href')).toBe(expectedPath);
    });

    it('should ignore hash characters', function() {
      var fakeAnchor = 'test';
      var el = angular.element('<a data-anchor="#' + fakeAnchor + '">Test</a>');

      $compile(el)($scope);
      $scope.$digest();

      var expectedPath = '#' + FAKE_PATH + '?section=' + fakeAnchor;

      expect(el.attr('href')).toBe(expectedPath);
    });
  });
}());
