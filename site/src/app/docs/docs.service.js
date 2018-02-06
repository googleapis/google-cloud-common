(function() {
  'use strict';

  angular
    .module('gcloud')
    .factory('DocsService', DocsService);

  /** @ngInject */
  function DocsService($sce, manifest, util) {
    function setAsTrusted(_method) {
      var method = angular.copy(_method);

      method.isConstructor = method.type === 'constructor';

      method.typeSymbol = getTypeSymbol(method.type);

      if (method.description) {
        method.description = $sce.trustAsHtml(method.description);
      }

      if (method.examples) {
        method.examples = method.examples.map(trustExample);
      }

      if (method.returns) {
        method.returns = method.returns.map(trustReturn);
      }

      if (method.params) {
        method.params = method.params.map(trustParam);
      }

      if (method.overview) {
        method.overview = $sce.trustAsHtml(method.overview);
      }

      return method;
    }

    function getTypeSymbol(methodType) {
      var typeSymbol = '#';
      if (methodType && manifest.methodTypeSymbols) {
        var mapping = util.findWhere(manifest.methodTypeSymbols, {
          type: methodType
        });
        if (mapping) {
          typeSymbol = mapping.symbol;
        }
      }
      return typeSymbol;
    }

    function trustReturn(returnValue) {
      return $sce.trustAsHtml([
        returnValue.types.join(', '),
        returnValue.description
      ].join(''));
    }

    function trustExample(example) {
      var MIN_CODE_LINES = 10;
      var useLineNumbers = false;
      var openTag = '<span class="code-block__line">';
      var closeTag = '</span>';
      var code, caption;

      if (example.code) {
        code = example.code.split('\n');
        useLineNumbers = code.length > MIN_CODE_LINES;
        code = openTag + code.join(closeTag + '\n' + openTag) + closeTag;
        code = $sce.trustAsHtml(code);
      }

      if (example.caption) {
        caption = $sce.trustAsHtml(example.caption);
      }

      return {
        code: code,
        caption: caption,
        useLineNumbers: useLineNumbers
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

    return {
      setAsTrusted: setAsTrusted,
      trust: $sce.trustAsHtml.bind($sce)
    };
  }

}());
