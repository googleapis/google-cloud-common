(function() {
  'use strict';

  describe('version-switcher', function() {
    var FAKE_MODULE_NAME = 'google-cloud-wat';
    var FAKE_TAG = 'bigquery-0.1.0';
    var FAKE_VERSIONS = [
      '0.36.0',
      '0.35.0',
      'master'
    ];

    var el;
    var $compile;
    var $scope;

    beforeEach(function() {
      module('gcloud.manifest', function($provide) {
        $provide.constant('manifest', {
          versions: []
        });
      });
      module('gcloud');
    });

    beforeEach(inject(function($rootScope, $injector) {
      $compile = $injector.get('$compile');
      $scope = $rootScope.$new();

      $scope.moduleName = FAKE_MODULE_NAME;

      $scope.docs = {
        selectedVersion: FAKE_VERSIONS[0],
        tagName: FAKE_TAG,
        versions: FAKE_VERSIONS
      };

      el = angular.element('<version-switcher></version-switcher>');
    }));

    it('should create a version switcher', function() {
      $compile(el)($scope);
      $scope.$digest();

      var options = el.find('select').children();

      expect(options.length).toBe(FAKE_VERSIONS.length);

      FAKE_VERSIONS.forEach(function(fakeVersion, i) {
        expect(options[i].innerText).toBe(fakeVersion);
      });
    });

    it('should show the release notes link for releases', function() {
      $compile(el)($scope);
      $scope.$digest();

      var expectedPath = 'https://github.com/GoogleCloudPlatform/' +
        FAKE_MODULE_NAME + '/releases/tag/' + FAKE_TAG;

      var releaseNotes = el.find('a');

      expect(releaseNotes.attr('href')).toBe(expectedPath);
    });

    it('should not show the release notes link for master', function() {
      $scope.docs.selectedVersion = 'master';

      $compile(el)($scope);
      $scope.$digest();

      var releaseNotes = el.find('a');

      expect(releaseNotes.length).toBe(0);
    });
  });
}());
