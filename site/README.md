# gcloud Shared Docs App

## Table of Contents

* [Installation][installation]
* [Previewing Locally][previewing]
  * [Previewing with `gh-pages`][previewing-ghpages]
* [Manifest][manifest]
  * [`lang`][lang-key]
  * [`friendlyLang`][friendlylang-key]
  * [`libraryTitle`][librarytitle-key]
  * [`defaultService`][defaultservice-key]
  * [`markdown`][markdown-key]
  * [`titleDelimiter`][delimiter-key]
  * [`versions`][versions-key]
  * [`content`][content-key]
  * [`home`][home-key]
  * [`package`][package-key]
  * [`methodTypeSymbols`][methodtypesymbols-key]
  * [`moduleName`][modulename-key]
  * [`defaultModule`][defaultmodule-key]
  * [`modules`][modules-key]
  * [`matchPartialServiceId`][partialserviceid-key]
* [Table of Contents][toc]
  * [`overview`][overview-key]
  * [`guides`][guides-key]
  * [`services`][services-key]
* [Types][types]
  * [`types`][types-key]
* [JSON Docs Schema][json-schema]
  * [`id`][id-key]
  * [`description` and `caption`][description-keys]
  * [Misc. keys][misc-keys]
  * [`methods`][methods-key]
    * [`params`][params-key]
    * [`exceptions`][exceptions-key]
    * [`returns`][returns-key]
    * [Misc. keys][misc-keys-1]
  * [Custom Data Types][custom-types]
* [Misc. Content][misc-content]
  * [Landing Page][landing-page]
  * [Overview Section][overview-section]
  * [Custom Page Headers][custom-page-headers]
* [Modular Docs][modular-docs]
* [Deploying][deploying]

## Installation

Install `bower` and `gulp` globally. You'll need to install [Node.js][nodejs] if it isn't already.

```sh
$ npm install -g gulp-cli bower
```

Clone the gcloud-common repository

```sh
$ git clone git@github.com:GoogleCloudPlatform/gcloud-common.git
$ cd gcloud-common/site
```

Install all the things!

```sh
$ bower install
$ npm install
```

Now we can start the local webserver via `gulp`. This will launch a browser window.

```sh
$ gulp serve
```

## Previewing Locally

In order to test locally you'll need a [<kbd>manifest.json</kbd>][manifest] file as well as content directory containing your [JSON docs][json-schema].

For testing purposes these should both be placed directly within the `src` directory.

Your <kbd>src</kbd> directory should then resemble something like the following

```sh
gcloud-common/site/src
 ├─── app # front-end application files
 ├─── assets # images, etc.
 ├─── versions # JSON content folder
 |     └─── v0.28.0 # one folder per release
 |           ├─── toc.json # table of contents for this version
 |           ├─── types.json # list of custom types for this version
 |           └─── storage # one folder per service
 |                 ├─── index.json # one file per class/module/etc.
 |                 └─── bucket.json
 ├─── index.html
 └─── manifest.json
```

For a living example, refer to gcloud-node's [gh-pages][gcloud-node-ghpages]. *Please note* their content directory is called <kbd>json</kbd>.

### Previewing with `gh-pages`

If you already have the docs app deployed to your gcloud library's `gh-pages` branch, this can make testing a whole lot easier. Since the application is already built, you can simply checkout the `gh-pages` branch and add your JSON to the <kbd>master</kbd> directory.

**Note:** some browsers may require you to disable web security to make local http requests, you can circumvent this by running a local web server.

## Manifest

The <kbd>manifest.json</kbd> is used to supply the Angular application with the necessary information to display your documentation properly. Please refer to the [schema][manifest-schema] for more information.

##### `lang` key

This is the non-friendly name for your language, it will be used to generate links to github, stackoverflow, etc. It's also used internally within the application for [language specific hooks][custom-page-headers]

```js
{
  "lang": "node"
}
```

##### `friendlyLang` key

This is the user-friendly version of your language name, in some cases it may be displayed to the user as a title or link text.

```js
{
  "friendlyLang": "Node.js"
}
```

##### `libraryTitle` key

*This key is completely optional*. Used for displaying the library title to users. Defaults to `gcloud` if not provided.

```js
{
  "libraryTitle": "the Google Cloud Client Library"
}
```

##### `defaultService` key

*This key is completely optional*. The custom service type to load by default. Defaults to `gcloud` if not provided.

```js
{
  "defaultService": "servicebuilder"
}
```

##### `markdown` key

The Angular app currently leverages a code highlighting library called [highlightjs][hljs]. The `markdown` field specifies the flavor of syntax highlighting to use.

```js
{
  "markdown": "javascript"
}
```

##### `titleDelimiter` key

*This key is completely optional*. Used for joining title strings together, if omitted the default value is `" » "`.

```js
{
  "titleDelimiter": " » "
}
```

For a complete list of available languages, please refer to the [highlightjs docs][hljs-languages].

##### `versions` key

This is a list of all available versions the user can switch between when browsing the documentation.

Each version should comply with [semver][semver]. The only exception to this rule is `master`.

```js
{
  "versions": [
    "v0.27.0",
    "v0.26.0",
    "master"
  ]
}
```

##### `content` key

This specifies the directory where your JSON docs reside.

```js
{
  "content": "versions"
}
```

##### `home` key

For the landing page of the documentation, it was decided (*see [gcloud-common#44](https://github.com/GoogleCloudPlatform/gcloud-common/issues/44)*) to simply use html files since they differ from language to language.

```js
{
  "home": "home.html"
}
```

See the [Landing Page section][landing-page] for more information.

##### `package` key

This allows you to specify what package manager you use and a direct link to your package.

```js
{
  "package": {
    "title": "npm",
    "href": "https://www.npmjs.com/package/gcloud"
  }
}
```

##### `methodTypeSymbols` key

This optional list allows you to map language-specific method type names (such as `instance` or `static`) to labels or symbols displayed in the method listing (such as `#`.) If not provided, the pound sign (`#`) will be used for all method types.

```js
{
  "methodTypeSymbols": [
    {
      "type": "class",
      "symbol": "::"
    },
    {
      "type": "instance",
      "symbol": "#"
    }
  ]
}
```

##### `moduleName`

The name of your module, this is used to create links to GitHub/Stackoverflow/etc.

```js
{
  "moduleName": "gcloud-node"
}
```

##### `defaultModule`

When using [modular docs][modular-docs], use this key to set the default module. The app will direct the user to this module in the event of an error and/or when the module has been omitted altogether.

The value should be the `id` of a module listed within the [`modules`][modules-key] array.

```js
{
  "defaultModule": "google-cloud"
}
```

##### `modules`

This field is only needed when using modular docs. When present it should contain a list of separate modules you wish to display docs for.

```js
{
  "modules": [
    {
      "id": "google-cloud", // id used for routing
      "name": "google-cloud", // name used for display purposes
      "defaultService": "storage", // default service page
      "versions": [ // available versions of the module
        "v0.29.0",
        "v0.10.0",
        "master"
      ]
    }
  ]
}
```

#### `matchPartialServiceId`

This is used for determining the logic used for collapsing and expanding the API navigation. By default any pages nested within a service `/bigquery/job` will check to see if the parent url is present within the nested url. If you decide to use a different URL layout, then you can set `matchPartialServiceId` to `true` and it will only checkout against the first item within the parent url.

Consider the following example:

If we're creating routes for our BigQuery service and instead of making the default page `/bigquery` we want to make it `/bigquery/client` - how do we know if something like `/bigquery/job` is a parent or sibling page to `/bigquery/client`? By setting `matchPartialServiceId` it will not check against the entire parent url - but instead just the first parameter within it.

```js
{
  "matchPartialServiceId": true
}
```

See the [JSON Docs Schema section][json-schema] for more information.

## Table of Contents

The <kbd>toc.json</kbd> file is used to create the primary navigation component, it also maps JSON content to custom data types that may not be accessible via main navigation.

For each version folder you have, there should be a corresponding <kbd>toc.json</kbd> file located within said folder.

##### `overview` key

**Note:** *This field is completely optional.*

When supplied, the overview field maps to an html file that resembles a quick-start guide.

```js
{
  "overview": "overview.html"
}
```

See the [Overview section][overview-section] for more information.

##### `guides` key

This will act as a table of contents for the *Getting Started* section of the documentation site.

Currently guides are composed of markdown files, you can include as many markdown files within a guide as you like. Any local guides detected will assume to be versioned, so <kbd>authentication.md</kbd> will essentially point to <kbd>versions/v0.28.0/authentication.md</kbd>.

Optionally you can also provide an `edit` field, which should map to a URL that allows the user to edit the guide content.

```js
{
  "guides": [{
    "title": "Authentication", // title of guide
    "id": "authentication", // id used for content lookup and generating guide url
    "edit": "https://github.com/GoogleCloudPlatform/gcloud-common/edit/master/authentication/readme.md",
    "contents": [
      "https://raw.githubusercontent.com/GoogleCloudPlatform/gcloud-common/master/authentication/readme.md",
      "authentication.md"
    ]
  }]
}
```

##### `services` key

The services section is very similar to the [guides section][guides-key]. This key will act as a table of contents for the *API* section of the documentation site. Instead of markdown, this section of the docs is driven by [JSON][json-schema].

The `type` key should map to an object within the [`types`][types-key] array.

You can also create nested sections for the various service concepts via `nav` key. The objects that occupy the `nav` array match that of the `service` array.

```js
{
  "services": [{
    "title": "Storage",
    "type": "storage",
    "nav": [{
      "title": "Bucket",
      "type": "storage/bucket"
    }]
  }]
}
```

## Types

The types section lists all the available data types for the application. All types should be placed within the <kbd>types.json</kbd> file.

##### `types` key

The `id` key will be used to map content from urls - `/docs/v0.28.0/storage/bucket` will map to `"id": "storage/bucket"`

The `title` key will be used to create a title on the service page. You can provide either a string or an array of strings that will be joined by a special character specified in the <kbd>manifest.json</kbd> via [`titleDelimiter`][delimiter-key].

```js
{
  "types": [{
    "title": "Storage",
    "id": "storage",
    "contents": "storage/index.json"
  }, {
    "title": ["Storage", "Bucket"],
    "id": "storage/bucket",
    "contents": "storage/bucket.json"
  }]
}
```

## JSON Docs Schema

Please refer to the [schema][json-schema] for more detailed information.

##### `id` key

This is the id for the service.

```js
{
  "id": "storage"
}
```

##### `description` and `caption` keys

Throughout the JSON structure there will be several instances of the `description` and `caption` keys, these should be wrapped in `<p>` tags.

```js
{
  "description": "<p>Hello, world!</p>"
}
```

##### Misc. keys

The top-level object contains a friendly name for the service. Optionally you can provide a service description and external links.

```js
{
  "name": "Storage",
  "description": "<p>Storage is cool!</p>", // optional
  "resources": [{ // optional
    "title": "Storage Overview",
    "link": "https://storage-is-cool.com"
  }]
}
```

##### `methods` key

This field defines all the methods available for this service. Methods contain metadata for a method, a list of parameters, a list of exceptions and possible return values.

```js
{
  "methods": [{
    "params": [],
    "exceptions": [],
    "returns": []
  }]
}
```

###### `params` key

This field is optional, when present it should contain a list of parameters that the method will accept.

```js
{
  "methods": [{
    "params": [{
      "name": "callback",
      "description": "<p>The callback function.</p>",
      "types": ["function"],
      "optional": true,
      "nullable": false
    }]
  }]
}
```

It's also possible to document nested keys or callback arguments by simply prefixing the key name with the object/callback name.

```js
{
  "name": "callback.err",
  "description": "<p>An error returned while making this request.</p>",
  "types": ["error"],
  "optional": false,
  "nullable": true
}
```

###### `exceptions` key

This field is optional, when present it should contain a list of exceptions that the method can throw.

```js
{
  "methods": [{
    "exceptions": [{
      "type": "Error", // error class
      "description": "<p>If a bucket ID is not provided.</p>" // why it was thrown
    }]
  }]
}
```

###### `returns` key

This field is optional, when present it should contain a list of return values.

```js
{
  "methods": [{
    "returns": [{
      "type": "Bucket",
      "description": "<p>A storage/bucket object</p>"
    }]
  }]
}
```

###### Misc. keys

This field is similar to the top-level metadata field, however it can also include several other items to better describe a method (examples, source code, etc.)

```js
{
  "methods": [{
    "type": "constructor", // we document constructors/modules in the same fashion as methods
    "id": "storage#createBucket", // unique ID for method
    "name": "createBucket", // method name -> Storage#createBucket
    "source": "/lib/storage/index.js#L270", // github path for deeplinking
    "description": "<p>Create a bucket.</p>",
    "examples": [{ // list of examples
      "caption": "<p>Here's how you would create a bucket!</p>", // caption for example
      "code": "storage.createBucket('new-bucket', callback);" // example code
    }],
    "resources": [{ // list of external resources
      "title": "Buckets: insert API docs",
      "link": "https://storage-is-cool.com/create-buckets/"
    }]
  }]
}
```

### Custom Data Types

In some cases you may need to link to custom data types, this can happen pretty much anywhere (descriptions, param types, return types, exception types, etc.). If this is functionality that you need, you can instead return an `<a>` tag using a custom data attribute `custom-type`.

```js
{
  "methods": [{
    "returns": [{
      "type": ["<a data-custom-type=\"storage/bucket\"></a>"],
      "description": "<p>A storage/bucket object</p>"
    }]
  }]
}
```

## Misc. Content

While majority of the content on the doc app is driven by markdown and content, there are a couple of other sections driven by html.

##### Landing Page

Specify a landing page using the <kbd>manifest.json</kbd>'s [`home` key][home-key].

For the landing page, you can create an html file that leverages Angular. There will be an object available to use within the template named `home`.

The home object will resemble the following

```js
{
  // information about the most recent release
  "latestRelease": {
    "name": "v0.28.0",
    "date": 1455306471454,
    "link": "https://github.com/GoogleCloudPlatform/gcloud-node/tree/v0.28.0"
  }
}
```

See a living example of gcloud-node's home template [here][gcloud-node-home].

##### Overview Section

Specify an overview using the <kbd>toc.json</kbd>'s [`overview` key][overview-key].

Overviews are completely optional, you can create one using html.

See [gcloud-node docs][gcloud-node-docs] for a working demo. The contents of the overview file fill the *Getting Started with gcloud* section of the service pages.

See a living example of gcloud-node's overview template [here][gcloud-node-overview].

## Modular Docs

If your library uses a modular approach (a separate installable package per service), you can optionally leverage the modular docs. To do this you must first make sure that your [manifest][manifest] contains the [`modules`][modules-key] and [`defaultModule`][defaultmodule-key] fields.

After which, you must create a folder within your content directory for each package. Each package should map to a module listed within your manifest. Here is an example of what your content folder should resemble

```sh
json
 ├─── bigquery #standalone package
 |     └─── master
 |           ├─── toc.json # table of contents for this version
 |           ├─── types.json # list of custom types for this version
 |           └─── index.json # # one file per class/module/etc.
 |
 └─── google-cloud # bundled package
       └─── v0.28.0
             ├─── toc.json # table of contents for this version
             ├─── types.json # list of custom types for this version
             └─── storage # one folder per service
                   ├─── index.json 
                   └─── bucket.json
```

## Deploying

Integrating the shared docs app is fairly painless. There is a [shell script][gcloud-common-deploy] that will push the updated site code to your `gh-pages` after a successful merge. To enable this, simply append a command to push to your repo to the script.

```sh
deploy_docs "googlecloudplatform/gcloud-node"
```

You'll probably also need to create a custom build script in your repository to generate the documentation and move it to the appropriate version folder, etc.

Please refer to gcloud-node's [`gh-pages` branch][gcloud-node-ghpages] for an example of how your content should be organized.

[installation]: #installation
[previewing]: #previewing-locally
[previewing-ghpages]: #previewing-with-gh-pages
[manifest]: #manifest
[lang-key]: #lang-key
[friendlylang-key]: #friendlylang-key
[librarytitle-key]: #librarytitle-key
[defaultservice-key]: #defaultservice-key
[markdown-key]: #markdown-key
[delimiter-key]: #titledelimiter-key
[versions-key]: #versions-key
[content-key]: #content-key
[home-key]: #home-key
[overview-key]: #overview-key
[guides-key]: #guides-key
[services-key]: #services-key
[package-key]: #package-key
[modulename-key]: #modulename-key
[defaultmodule-key]: #defaultmodule-key
[modules-key]: #modules-key
[partialserviceid-key]: #matchpartialserviceid
[json-schema]: #json-docs-schema
[id-key]: #id-key
[description-keys]: #description-and-caption-keys
[misc-keys]: #misc-keys
[methods-key]: #methods-key
[misc-keys-1]: #misc-keys-1
[params-key]: #params-key
[exceptions-key]: #exceptions-key
[returns-key]: #returns-key
[custom-types]: #custom-data-types
[misc-content]: #misc-content
[landing-page]: #landing-page
[overview-section]: #overview-section
[custom-page-headers]: #custom-page-headers
[deploying]: #deploying
[toc]: #table-of-contents
[types]: #types
[types-key]: #types-key
[methodtypesymbols-key]: #methodtypesymbols-key
[modular-docs]: #modular-docs

[nodejs]: https://nodejs.org/en/
[hljs]: https://highlightjs.org/
[hljs-languages]: http://highlightjs.readthedocs.org/en/latest/css-classes-reference.html#language-names-and-aliases
[semver]: http://semver.org/

[gcloud-node-ghpages]: https://github.com/GoogleCloudPlatform/gcloud-node/tree/gh-pages
[gcloud-node-home]: https://github.com/GoogleCloudPlatform/gcloud-node/blob/gh-pages/home.html
[gcloud-node-overview]: https://github.com/GoogleCloudPlatform/gcloud-node/blob/gh-pages/json/v0.28.0/overview.html
[gcloud-node-docs]: https://googlecloudplatform.github.io/gcloud-node/#/docs/latest/
[gcloud-common-deploy]: https://github.com/GoogleCloudPlatform/gcloud-common/blob/master/deploy-docs.sh#L71
[manifest-schema]: /site/schemas/manifest.schema.json
[json-schema]: /site/schemas/service.schema.json
[page-header-template]: https://github.com/GoogleCloudPlatform/gcloud-common/blob/master/site/src/app/components/page-header/page-header.directive.js#L8
