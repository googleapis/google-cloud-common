(function() {
  'use strict';

  angular
    .module('gcloud')
    .controller('DocsCtrl', DocsCtrl);

  /** @ngInject */
  function DocsCtrl($state, langs, manifest, toc, lastBuiltDate) {
    var docs = this;

    docs.libraryTitle = manifest.libraryTitle || 'gcloud';
    docs.langs = langs;
    docs.lastBuiltDate = lastBuiltDate;
    docs.guides = toc.guides;
    docs.services = toc.services;
    docs.version = $state.params.version;
    docs.overviewFileUrl = null;

    docs.selectedVersion = docs.version;
    docs.loadVersion = loadVersion;
    docs.getGuideUrl = getGuideUrl;
    docs.isActive = isActive;

    if (toc.overview) {
      docs.overviewFileUrl = [
        manifest.content,
        $state.params.version,
        toc.overview
      ].join('/');
    }

    function loadVersion(version) {
      return $state.go($state.current.name, { version: version });
    }

    function isActive(serviceId) {
      serviceId = serviceId.split('/')[0];
      return !!($state.params.serviceId || '').match(serviceId);
    }

    function getGuideUrl(page) {
      return page.title.toLowerCase().replace(/\s/g, '-');
    }
  }
}());
