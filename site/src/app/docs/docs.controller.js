(function() {
  'use strict';

  angular
    .module('gcloud')
    .controller('DocsCtrl', DocsCtrl);

  /** @ngInject */
  function DocsCtrl($state, langs, manifest, toc, types, lastBuiltDate, versions) {
    var docs = this;

    docs.libraryTitle = manifest.libraryTitle || 'Google Cloud';
    docs.langs = langs;
    docs.lastBuiltDate = lastBuiltDate;
    docs.guides = toc.guides;
    docs.services = toc.services;
    docs.versions = versions;
    docs.version = $state.params.version;
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

    function isActive(serviceId) {
      var serviceIdParam = $state.params.serviceId || '';
      var isCompleteMatch = !!serviceIdParam.match(serviceId);

      var partialServiceId = serviceId.split('/')[0];
      var isPartialMatch = !!serviceIdParam.match(partialServiceId);

      return isCompleteMatch || isPartialMatch;
    }

    function getGuideUrl(page) {
      return page.title.toLowerCase().replace(/\s/g, '-');
    }
  }
}());
