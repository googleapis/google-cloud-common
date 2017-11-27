(function() {
  'use strict';

  angular
    .module('gcloud')
    .controller('DocsCtrl', DocsCtrl);

  /** @ngInject */
  function DocsCtrl($state, langs, manifest, toc, types, lastBuiltDate, versions, util) {
    var docs = this;

    docs.libraryTitle = manifest.libraryTitle || 'Google Cloud';
    docs.langs = langs;
    docs.lastBuiltDate = lastBuiltDate;
    docs.latestDocsUrl = getNewDocsUrl();
    docs.guides = toc.guides;
    docs.services = toc.services;
    docs.versions = versions;
    docs.version = $state.params.version;
    docs.tagName = toc.tagName || docs.version;
    docs.overviewFileUrl = null;
    docs.types = types;

    docs.selectedVersion = docs.version;
    docs.loadVersion = loadVersion;
    docs.getGuideUrl = getGuideUrl;
    docs.isActive = isActive;

    if (toc.overview) {
      docs.overviewFileUrl = [
        manifest.content,
        $state.params.module,
        $state.params.version,
        toc.overview
      ].join('/');
    }

    function loadVersion(version) {
      return $state.go($state.current.name, { version: version });
    }

    function isActive(service) {
      var serviceId = service.type;
      var serviceIdParam = $state.params.serviceId || '';

      // Match the first element in the type. E.g., nav parent
      // `datastore/client` matches `datastore` extracted from nav child
      // `datastore/query`.
      if (manifest.matchPartialServiceId) {
        var partialServiceId = serviceId.split('/')[0];
        return !!serviceIdParam.match(partialServiceId);

      // If toc entry contains 'patterns', attempt to match any of them.
      // E.g., both nav parent `google/cloud/datastore` and nav child
      // `google/datastore/v1` match the pattern `datastore`.
      } else if (service.patterns)  {
        var matched = false;
        angular.forEach(service.patterns, function(pattern) {
          if (!matched) { // Simply skip if already matched
            if (new RegExp(pattern).test(serviceIdParam)) {
              matched = true;
            }
          }
        });
        return matched;
      }
      // Match using type. E.g., `datastore/query` is a match for `datastore`.
      return !!serviceIdParam.match(serviceId);
    }

    function getGuideUrl(page) {
      return page.title.toLowerCase().replace(/\s/g, '-');
    }

    function getNewDocsUrl() {
      if (manifest.latestDocsUrl) {
        return manifest.latestDocsUrl;
      }

      if (manifest.modules) {
        var mod = util.findWhere(manifest.modules, {
          id: $state.params.module
        });

        if (mod) {
          return mod.latestDocsUrl;
        }
      }

      return null;
    }
  }
}());
