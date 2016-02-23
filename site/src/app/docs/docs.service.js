/* global hljs:true */
(function() {
  'use strict';

  angular
    .module('gcloud')
    .factory('DocsService', DocsService);

  /** @ngInject */
  function DocsService($sce, manifest) {
    function setAsTrusted(_method) {
      var method = angular.copy(_method);

      if (method.metadata) {
        method.metadata.description = $sce.trustAsHtml(method.metadata.description);
        method.metadata.isConstructor = method.metadata.constructor === true;
        method.metadata.anchor = getMethodAnchor(method.metadata);
        method.metadata.typeSymbol = getMethodTypeSymbol(method.metadata.type);

        if (method.metadata.examples) {
          method.metadata.examples = method.metadata.examples.map(trustExample);
        }
      }

      if (method.returns) {
        method.returns = method.returns.map(trustReturn);
      }

      if (method.params) {
        method.params = method.params.map(trustParam);
      }

      return method;
    }

    function trustReturn(returnValue) {
      return $sce.trustAsHtml(returnValue.types.join(', '));
    }

    function trustExample(example) {
      var code, caption;

      if (example.code) {
        code = hljs.highlight(manifest.markdown, example.code);
        code = $sce.trustAsHtml(code.value);
      }

      if (example.caption) {
        caption = $sce.trustAsHtml(example.caption);
      }

      return {
        code: code,
        caption: caption
      };
    }

    function trustParam(param) {
      var name = param.name.split('.');

      if (name.length > 1) {
        param.name = name.pop();
        param.parent = name.join('.');
      }

      param.types = $sce.trustAsHtml(param.types.join(', '));
      param.description = $sce.trustAsHtml(param.description);

      return param;
    }

    function getMethodAnchor(metadata) {
      var anchor = metadata.name;
      if (metadata.type) {
        anchor += "-type-" + metadata.type;
      }
      return anchor;
    }

    function getMethodTypeSymbol(type) {
      var typeSymbol = "#";
      if (type === "class") {
        typeSymbol = ".";
      }
      return typeSymbol;
    }

    return {
      setAsTrusted: setAsTrusted
    };
  }

}());
