(function() {
  'use strict';

  angular
    .module('gcloud')
    .directive('anchor', anchorDirective);

  /** @ngInject */
  function anchorDirective($location) {
    return {
      restrict: 'A',
      link: function(scope, elem, attrs) {
        var anchor = attrs.anchor.replace(/^\#/, '');
        var href = '#' + $location.path() + '?section=' + anchor;

        elem.attr('href', href);
      }
    };
  }

}());
