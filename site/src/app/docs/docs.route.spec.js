(function() {

  'use strict';

  describe('routing', function() {
    var $location;
    var $rootScope;

    var DEFAULT_MOD = 'google-cloud';
    var LATEST = 'v0.39.0';

    function goTo(path) {
      $location.path(path);
      $rootScope.$emit('$locationChangeSuccess');
    }

    angular.module('gcloud.manifest', []);

    describe('modular', function() {
      beforeEach(function() {
        module('gcloud.manifest', function($provide) {
          $provide.constant('manifest', {
            moduleName: 'gcloud-node',
            defaultModule: DEFAULT_MOD,
            modules: [{
              id: DEFAULT_MOD,
              defaultService: DEFAULT_MOD,
              versions: [
                LATEST,
                '0.38.0'
              ]
            }]
          });
        });
        module('gcloud');
      });

      beforeEach(inject(function($injector) {
        $location = $injector.get('$location');
        $rootScope = $injector.get('$rootScope');
      }));

      it('should redirect to the latest docs', function() {
        goTo('/docs');
        expect($location.path()).toBe('/docs/latest');
      });

      it('should redirect to the home page', function() {
        goTo('/never/gonna/give/you/up');
        expect($location.path()).toBe('/');
      });

      it('should send the user to the default module', function() {
        goTo('/docs/latest');
        expect($location.path()).toBe('/docs/google-cloud/latest');
      });

      it('should redirect the user from a bad module to 404', function() {
        goTo('/docs/notreallyathing/latest');
        expect($location.path()).toBe('/docs/google-cloud/v0.39.0/not-found');
      });

      it('should replace the "latest" alias', function() {
        goTo('/docs/google-cloud/latest');
        expect($location.path()).toBe('/docs/google-cloud/v0.39.0');
      });

      it('should replace the "stable" alias', function() {
        goTo('/docs/google-cloud/stable');
        expect($location.path()).toBe('/docs/google-cloud/v0.39.0');
      });

      it('should redirect the user from a bad version to 404', function() {
        goTo('/docs/google-cloud/v0.1.0');
        expect($location.path()).toBe('/docs/google-cloud/v0.39.0/not-found');
      });

      it('should strip the "v" from the version if need be', function() {
        goTo('/docs/google-cloud/v0.38.0');

        expect($location.path())
          .toBe('/docs/google-cloud/0.38.0');
      });

      it('should send the user to the default service', function() {
        goTo('/docs/google-cloud/v0.39.0');

        expect($location.path())
          .toBe('/docs/google-cloud/v0.39.0/google-cloud');
      });

      it('should retain url params when redirecting', function() {
        goTo('/docs/google-cloud/latest/bigtable/row');

        expect($location.path())
          .toBe('/docs/google-cloud/v0.39.0/bigtable/row');
      });

      it('should retain url params when version is omitted', function() {
        goTo('/docs/google-cloud/bigtable/row');

        expect($location.path())
          .toBe('/docs/google-cloud/v0.39.0/bigtable/row');
      });

      it('should retain url params when module is omitted', function() {
        goTo('/docs/v0.39.0/bigtable/row');

        expect($location.path())
          .toBe('/docs/google-cloud/v0.39.0/bigtable/row');
      });
    });

    describe('singular', function() {
      beforeEach(function() {
        module('gcloud.manifest', function($provide) {
          $provide.constant('manifest', {
            moduleName: 'gcloud-node',
            defaultService: DEFAULT_MOD,
            versions: [ LATEST ]
          });
        });
        module('gcloud');
      });

      beforeEach(inject(function($injector) {
        $location = $injector.get('$location');
        $rootScope = $injector.get('$rootScope');
      }));

      it('should replace the "latest" alias', function() {
        goTo('/docs/latest/google-cloud');
        expect($location.path()).toBe('/docs/v0.39.0/google-cloud');
      });

      it('should replace the "stable" alias', function() {
        goTo('/docs/stable/google-cloud');
        expect($location.path()).toBe('/docs/v0.39.0/google-cloud');
      });

      it('should add the version if omitted', function() {
        goTo('/docs/guides/authentication');
        expect($location.path()).toBe('/docs/v0.39.0/guides/authentication');
      });

      it('should redirect the user to the default service', function() {
        goTo('/docs/v0.39.0');
        expect($location.path()).toBe('/docs/v0.39.0/google-cloud');
      });
    });
  });

}());
