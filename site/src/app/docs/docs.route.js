(function() {
  'use strict';

  angular
    .module('gcloud')
    .config(docsRoutes);

  /** @ngInject */
  function docsRoutes($stateProvider, $urlRouterProvider, $urlMatcherFactoryProvider, manifest) {
    // source: https://github.com/sindresorhus/semver-regex
    var regSemver = '\\bv?(?:0|[1-9][0-9]*)\\.(?:0|[1-9][0-9]*)\\.(?:0|[1-9][0-9]*)(?:-[\\da-z\-]+(?:\\.[\\da-z\\-]+)*)?(?:\\+[\\da-z\\-]+(?:\\.[\\da-z\\-]+)*)?\\b';
    var baseVersionUrl = '/docs/{version:master|' + regSemver + '}';
    var latestVersion = manifest.versions[0];
    var defaultService = manifest.defaultService || 'gcloud';

    $urlMatcherFactoryProvider.type('nonURIEncoded', {
      encode: toString,
      decode: toString,
      is: function() { return true; }
    });

    $stateProvider
      .state('docs', {
        // ui-router allows one to optionally use a regular expressions to
        // match uri parameters against. In this instance we're only allowing
        // the version value to be "master" OR a valid semver version.
        // If we receive a route that does NOT match one of these two values
        // then we'll end up in the $urlRouterProvier.otherwise(...) block
        // which will look for version alias's or the absense of a version
        // and redirect the user appropriately
        url: baseVersionUrl,
        templateUrl: 'app/docs/docs.html',
        controller: 'DocsCtrl',
        controllerAs: 'docs',
        abstract: true,
        resolve: {
          lastBuiltDate: getLastBuiltDate,
          toc: getToc,
          types: getTypes
        },
        params: {
          version: latestVersion
        }
      })
      .state('docs.guides', {
        url: '/guides/:guideId?section',
        templateUrl: 'app/guide/guide.html',
        controller: 'GuideCtrl',
        controllerAs: 'guide',
        resolve: { guideObject: getGuide }
      })
      .state('docs.service', {
        url: '/{serviceId:nonURIEncoded}?method',
        templateUrl: 'app/service/service.html',
        controller: 'ServiceCtrl',
        controllerAs: 'service',
        resolve: { serviceObject: getService },
        params: { serviceId: 'gcloud' }
      });

    $urlRouterProvider.when('/docs', '/docs/latest');
    $urlRouterProvider.when(baseVersionUrl, '/docs/:version/' + defaultService);

    $urlRouterProvider.otherwise(function($injector, $location) {
      var path = $location.path();
      var docsBaseUrl = '/docs/';
      var isUnknownRoute = path.indexOf(docsBaseUrl) === -1;

      if (isUnknownRoute) {
        return '/';
      }

      var versions = $injector.get('manifest').versions;
      var params = path.replace(docsBaseUrl, '').split('/');
      var isValidVersion = versions.indexOf(params[0]) !== -1;

      // could be a bad service name
      if (isValidVersion) {
        return docsBaseUrl + params[0];
      }

      // could be a version alias
      if (params[0] === 'latest' || params[0] === 'stable') {
        params[0] = latestVersion;
      } else {
        // otherwise let's assume the version was omitted entirely
        params.unshift($injector.get('$stateParams').version || latestVersion);
      }

      return docsBaseUrl + params.join('/');
    });
  }

  /** @ngInject */
  function getLastBuiltDate($http, manifest) {
    var url = 'https://api.github.com/repos/GoogleCloudPlatform/gcloud-' +
      manifest.lang + '/commits?sha=gh-pages&per_page=1';

    return $http({
      method: 'get',
      url: url,
      cache: true
    })
    .then(function(resp) {
      return resp.data[0].commit.committer.date;
    })
    .then(null, angular.noop);
  }

  /** @ngInject */
  function getToc($interpolate, $http, $stateParams, manifest) {
    var tocUrl = $interpolate('{{content}}/{{version}}/toc.json')({
      content: manifest.content,
      version: $stateParams.version
    });

    return $http.get(tocUrl).then(function(response) {
      return response.data;
    });
  }

  /** @ngInject */
  function getTypes($interpolate, $http, $stateParams, manifest) {
    var typeUrl = $interpolate('{{content}}/{{version}}/types.json')({
      content: manifest.content,
      version: $stateParams.version
    });

    return $http.get(typeUrl).then(function(response) {
      return response.data;
    });
  }

  /** @ngInject */
  function getGuide($q, $stateParams, util, toc) {
    var guideId = $stateParams.guideId.replace(/\-/g, ' ');
    var guide = util.findWhere(toc.guides, { id: guideId });

    if (!guide) {
      return $q.reject('Unknown guide: ' + guideId);
    }

    return guide;
  }

  /** @ngInject */
  function getService($stateParams, $interpolate, $http, $q, manifest, types, util) {
    var serviceId = $stateParams.serviceId;
    var service = util.findWhere(types, {
      id: serviceId
    });

    if (!service) {
      return $q.reject('Unknown service: ' + serviceId);
    }

    var json = $interpolate('{{content}}/{{version}}/{{resource}}')({
      content: manifest.content,
      version: $stateParams.version,
      resource: service.contents
    });

    return $http.get(json).then(function(response) {
      var data = response.data;

      if (service.title) {
        data.title = service.title;
      }

      return data;
    });
  }

  function toString(val) {
    return val ? val.toString() : null;
  }

}());
