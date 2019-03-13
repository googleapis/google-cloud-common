package main

import (
	"fmt"
	"os"

	"github.com/golang/protobuf/jsonpb"
	pb "github.com/googleapis/google-cloud-common/testing/storage/genproto"
)

var testcases = []pb.SignedV4Test{
	{
		FileName: "simple-get",
		Description: "Simple GET",
		Bucket: "test-bucket",
		Object: "test-object",
		Method: "GET",
		Expiration: 10,
		Timestamp: "20190201T090000Z",
		ExpectedUrl: "https://storage.googleapis.com/test-bucket/test-object?X-Goog-Algorithm=GOOG4-RSA-SHA256&X-Goog-Credential=test-iam-credentials%40dummy-project-id.iam.gserviceaccount.com%2F20190201%2Fauto%2Fstorage%2Fgoog4_request&X-Goog-Date=20190201T090000Z&X-Goog-Expires=10&X-Goog-SignedHeaders=host&X-Goog-Signature=95e6a13d43a1d1962e667f17397f2b80ac9bdd1669210d5e08e0135df9dff4e56113485dbe429ca2266487b9d1796ebdee2d7cf682a6ef3bb9fbb4c351686fba90d7b621cf1c4eb1fdf126460dd25fa0837dfdde0a9fd98662ce60844c458448fb2b352c203d9969cb74efa4bdb742287744a4f2308afa4af0e0773f55e32e92973619249214b97283b2daa14195244444e33f938138d1e5f561088ce8011f4986dda33a556412594db7c12fc40e1ff3f1bedeb7a42f5bcda0b9567f17f65855f65071fabb88ea12371877f3f77f10e1466fff6ff6973b74a933322ff0949ce357e20abe96c3dd5cfab42c9c83e740a4d32b9e11e146f0eb3404d2e975896f74",
	},

	{
		FileName: "simple-put",
		Description: "Simple PUT",
		Bucket: "test-bucket",
		Object: "test-object",
		Method: "PUT",
		Expiration: 10,
		Timestamp: "20190201T090000Z",
		ExpectedUrl: "https://storage.googleapis.com/test-bucket/test-object?X-Goog-Algorithm=GOOG4-RSA-SHA256&X-Goog-Credential=test-iam-credentials%40dummy-project-id.iam.gserviceaccount.com%2F20190201%2Fauto%2Fstorage%2Fgoog4_request&X-Goog-Date=20190201T090000Z&X-Goog-Expires=10&X-Goog-SignedHeaders=host&X-Goog-Signature=8adff1d4285739e31aa68e73767a46bc5511fde377497dbe08481bf5ceb34e29cc9a59921748d8ec3dd4085b7e9b7772a952afedfcdaecb3ae8352275b8b7c867f204e3db85076220a3127a8a9589302fc1181eae13b9b7fe41109ec8cdc93c1e8bac2d7a0cc32a109ca02d06957211326563ab3d3e678a0ba296e298b5fc5e14593c99d444c94724cc4be97015dbff1dca377b508fa0cb7169195de98d0e4ac96c42b918d28c8d92d33e1bd125ce0fb3cd7ad2c45dae65c22628378f6584971b8bf3945b26f2611eb651e9b6a8648970c1ecf386bb71327b082e7296c4e1ee2fc0bdd8983da80af375c817fb1ad491d0bc22c0f51dba0d66e2cffbc90803e47",
	},

	{
		FileName: "post-for-resumable-uploads",
		Description: "POST for resumable uploads",
		Bucket: "test-bucket",
		Object: "test-object",
		Method: "POST",
		Expiration: 10,
		Headers: map[string]*pb.HeaderList{
			"M-goog-resumable": {Headers: []string{"start"}},
		},
		Timestamp: "20190201T090000Z",
		ExpectedUrl: "https://storage.googleapis.com/test-bucket/test-object?X-Goog-Algorithm=GOOG4-RSA-SHA256&X-Goog-Credential=test-iam-credentials%40dummy-project-id.iam.gserviceaccount.com%2F20190201%2Fauto%2Fstorage%2Fgoog4_request&X-Goog-Date=20190201T090000Z&X-Goog-Expires=10&X-Goog-SignedHeaders=host%3Bx-goog-resumable&X-Goog-Signature=4a6d39b23343cedf4c30782aed4b384001828c79ffa3a080a481ea01a640dea0a0ceb58d67a12cef3b243c3f036bb3799c6ee88e8db3eaf7d0bdd4b70a228d0736e07eaa1ee076aff5c6ce09dff1f1f03a0d8ead0d2893408dd3604fdabff553aa6d7af2da67cdba6790006a70240f96717b98f1a6ccb24f00940749599be7ef72aaa5358db63ddd54b2de9e2d6d6a586eac4fe25f36d86fc6ab150418e9c6fa01b732cded226c6d62fc95b72473a4cc55a8257482583fe66d9ab6ede909eb41516a8690946c3e87b0f2052eb0e97e012a14b2f721c42e6e19b8a1cd5658ea36264f10b9b1ada66b8ed5bf7ed7d1708377ac6e5fe608ae361fb594d2e5b24c54",
	},

	{
		FileName: "vary-expiration-and-timestamp",
		Description: "Vary expiration and timestamp",
		Bucket: "test-bucket",
		Object: "test-object",
		Method: "GET",
		Expiration: 20,
		Timestamp: "20190301T090000Z",
		ExpectedUrl: "https://storage.googleapis.com/test-bucket/test-object?X-Goog-Algorithm=GOOG4-RSA-SHA256&X-Goog-Credential=test-iam-credentials%40dummy-project-id.iam.gserviceaccount.com%2F20190301%2Fauto%2Fstorage%2Fgoog4_request&X-Goog-Date=20190301T090000Z&X-Goog-Expires=20&X-Goog-SignedHeaders=host&X-Goog-Signature=9669ed5b10664dc594c758296580662912cf4bcc5a4ba0b6bf055bcbf6f34eed7bdad664f534962174a924741a0c273a4f67bc1847cef20192a6beab44223bd9d4fbbd749c407b79997598c30f82ddc269ff47ec09fa3afe74e00616d438df0d96a7d8ad0adacfad1dc3286f864d924fe919fb0dce45d3d975c5afe8e13af2db9cc37ba77835f92f7669b61e94c6d562196c1274529e76cfff1564cc2cad7d5387dc8e12f7a5dfd925685fe92c30b43709eee29fa2f66067472cee5423d1a3a4182fe8cea75c9329d181dc6acad7c393cd04f8bf5bc0515127d8ebd65d80c08e19ad03316053ea60033fd1b1fd85a69c576415da3bf0a3718d9ea6d03e0d66f0",
	},

	{
		FileName: "vary-bucket-and-object",
		Description: "Vary bucket and object",
		Bucket: "test-bucket2",
		Object: "test-object2",
		Method: "GET",
		Expiration: 10,
		Timestamp: "20190201T090000Z",
		ExpectedUrl: "https://storage.googleapis.com/test-bucket2/test-object2?X-Goog-Algorithm=GOOG4-RSA-SHA256&X-Goog-Credential=test-iam-credentials%40dummy-project-id.iam.gserviceaccount.com%2F20190201%2Fauto%2Fstorage%2Fgoog4_request&X-Goog-Date=20190201T090000Z&X-Goog-Expires=10&X-Goog-SignedHeaders=host&X-Goog-Signature=36e3d58dfd3ec1d2dd2f24b5ee372a71e811ffaa2162a2b871d26728d0354270bc116face87127532969c4a3967ed05b7309af741e19c7202f3167aa8c2ac420b61417d6451442bb91d7c822cd17be8783f01e05372769c88913561d27e6660dd8259f0081a71f831be6c50283626cbf04494ac10c394b29bb3bce74ab91548f58a37118a452693cf0483d77561fc9cac8f1765d2c724994cca46a83517a10157ee0347a233a2aaeae6e6ab5e204ff8fc5f54f90a3efdb8301d9fff5475d58cd05b181affd657f48203f4fb133c3a3d355b8eefbd10d5a0a5fd70d06e9515460ad74e22334b2cba4b29cae4f6f285cdb92d8f3126d7a1479ca3bdb69c207d860",
	},

	{
		FileName: "simple-headers",
		Description: "Simple headers",
		Bucket: "test-bucket",
		Object: "test-object",
		Headers: map[string]*pb.HeaderList{
			"foo": {Headers: []string{"foo-value"}},
			"BAR": {Headers: []string{"BAR-value"}},
		},
		Method: "GET",
		Expiration: 10,
		Timestamp: "20190201T090000Z",
		ExpectedUrl: "https://storage.googleapis.com/test-bucket/test-object?X-Goog-Algorithm=GOOG4-RSA-SHA256&X-Goog-Credential=test-iam-credentials%40dummy-project-id.iam.gserviceaccount.com%2F20190201%2Fauto%2Fstorage%2Fgoog4_request&X-Goog-Date=20190201T090000Z&X-Goog-Expires=10&X-Goog-SignedHeaders=bar%3Bfoo%3Bhost&X-Goog-Signature=68ecd3b008328ed30d91e2fe37444ed7b9b03f28ed4424555b5161980531ef87db1c3a5bc0265aad5640af30f96014c94fb2dba7479c41bfe1c020eb90c0c6d387d4dd09d4a5df8b60ea50eb6b01cdd786a1e37020f5f95eb8f9b6cd3f65a1f8a8a65c9fcb61ea662959efd9cd73b683f8d8804ef4d6d9b2852419b013368842731359d7f9e6d1139032ceca75d5e67cee5fd0192ea2125e5f2955d38d3d50cf116f3a52e6a62de77f6207f5b95aaa1d7d0f8a46de89ea72e7ea30f21286318d7eba0142232b0deb3a1dc9e1e812a981c66b5ffda3c6b01a8a9d113155792309fd53a3acfd054ca7776e8eec28c26480cd1e3c812f67f91d14217f39a606669d",
	},

	// Note: some platforms may not expose multi-value headers to clients. They could skip
	// this test or perform the concatenation of header values themselves.
	{
		FileName: "multi-value-headers",
		Description: "Multi-value headers",
		Bucket: "test-bucket",
		Object: "test-object",
		Headers: map[string]*pb.HeaderList{
			"foo": {Headers: []string{"foo-value1", "foo-value2"}},
			"bar": {Headers: []string{"bar-value1", "bar-value2"}},
		},
		Method: "GET",
		Expiration: 10,
		Timestamp: "20190201T090000Z",
		ExpectedUrl: "https://storage.googleapis.com/test-bucket/test-object?X-Goog-Algorithm=GOOG4-RSA-SHA256&X-Goog-Credential=test-iam-credentials%40dummy-project-id.iam.gserviceaccount.com%2F20190201%2Fauto%2Fstorage%2Fgoog4_request&X-Goog-Date=20190201T090000Z&X-Goog-Expires=10&X-Goog-SignedHeaders=bar%3Bfoo%3Bhost&X-Goog-Signature=84a14ff388457290bc3ed7bfeb4745a1c2287e58965457d9d9959326fc2cbdfbb9128b6a002e86d617cb1d2187e3e075de223489d4e91418de76e21d4e561c618bc13ac72e1cc3b4e0c9eee880be577c417eb4623347d3d1ffd2a0705ab70bab6786f67107d05dc4652f2b84531dc01a15efa9ee3fbe504f6e76e64658fd1df431bf671a997db8ef7371eae8abbcc2690c085407738e32f396d9b87d0e974740ee0b7a256fc8471db27a6b554527b96dbd972073b89f57d6486182816b0d307875f1753bf16140332c6116899447769dd9f1985a520ca6ab50c614a80b3619e9d9a81ff81a6c14f51f1cf487243c2708aa9064e30acb5694af04c3fe0f5fd5a4",
	},

	{
		FileName: "headers-should-be-trimmed",
		Description: "Headers should be trimmed",
		Bucket: "test-bucket",
		Object: "test-object",
		Headers: map[string]*pb.HeaderList{
			"leading": {Headers: []string{"    xyz"}},
			"trailing": {Headers: []string{"abc    "}},
			"collapsed": {Headers: []string{"abc    def"}},
		},
		Method: "GET",
		Expiration: 10,
		Timestamp: "20190201T090000Z",
		ExpectedUrl: "https://storage.googleapis.com/test-bucket/test-object?X-Goog-Algorithm=GOOG4-RSA-SHA256&X-Goog-Credential=test-iam-credentials%40dummy-project-id.iam.gserviceaccount.com%2F20190201%2Fauto%2Fstorage%2Fgoog4_request&X-Goog-Date=20190201T090000Z&X-Goog-Expires=10&X-Goog-SignedHeaders=collapsed%3Bhost%3Bleading%3Btrailing&X-Goog-Signature=1839511d6238d9ac2bbcbba8b23515b3757db35dfa7b8f9bc4b8b4aa270224df747c812526f1a3bcf294d67ed84cd14e074c36bc090e0a542782934a7c925af4a5ea68123e97533704ce8b08ccdf5fe6b412f89c9fc4de243e29abdb098382c5672188ee3f6fef7131413e252c78e7a35658825ad842a50609e9cc463731e17284ff7a14824c989f87cef22fb99dfec20cfeed69d8b3a08f00b43b8284eecd535e50e982b05cd74c5750cd5f986cfc21a2a05f7f3ab7fc31bd684ed1b823b64d29281e923fc6580c49005552ca19c253de087d9d2df881144e44eda40965cfdb4889bf3a35553c9809f4ed20b8355be481b92b9618952b6a04f3017b36053e15",
	},

	// Separated from "Headers should be trimmed" test so it can be skipped on single-header-value-only platforms.
	{
		FileName: "trimming-of-multiple-headers",
		Description: "Trimming of multiple header values",
		Bucket: "test-bucket",
		Object: "test-object",
		Headers: map[string]*pb.HeaderList{
			"foo": {Headers: []string{"  abc  ", "  def  ", "  ghi  jkl  "}},
		},
		Method: "GET",
		Expiration: 10,
		Timestamp: "20190201T090000Z",
		ExpectedUrl: "https://storage.googleapis.com/test-bucket/test-object?X-Goog-Algorithm=GOOG4-RSA-SHA256&X-Goog-Credential=test-iam-credentials%40dummy-project-id.iam.gserviceaccount.com%2F20190201%2Fauto%2Fstorage%2Fgoog4_request&X-Goog-Date=20190201T090000Z&X-Goog-Expires=10&X-Goog-SignedHeaders=foo%3Bhost&X-Goog-Signature=96e36a82dd79e6d37070b5dfaffc616e8c5159c583261dd3858c2241c2a34f270f4fe2bf55ba6877a7c982f34b0b9114683ba37880e3ec378942972882dbcb99c6463573178c6167acc40b2be8db7f3a320de47373c30626a37fe9e6cc719ee6060f573bf1a30ef5e86338e834494c089226bef3722bf8ae2fa3a7599916bec92df30cf25852c3514e3be0f4541063cea2babf4825b8e38876454f1502f5e307d32381aa927113104a75c82a23f7e9597016ca0bc4971d5990515df2a0239a62c711d3aacea50b8e05106ae2a14201bd6dae369334c27fad5c14dac66103c5c1a980b3de263e85fe715010e603a518eaf6286b7beb24ca84b97752485c423f0a",
	},

	// Headers associated with customer-supplied encryption keys should not be included in the signature
	{
		FileName: "customer-supplied-encryption-key",
		Description: "Customer-supplied encryption key",
		Bucket: "test-bucket",
		Object: "test-object",
		Headers: map[string]*pb.HeaderList{
			"X-Goog-Encryption-Key": {Headers: []string{"ignored"}},
			"X-Goog-Encryption-Key-Sha256": {Headers: []string{"ignored"}},
			"X-Goog-Encryption-Algorithm": {Headers: []string{"ignored"}},
		},
		Method: "GET",
		Expiration: 10,
		Timestamp: "20190201T090000Z",
		ExpectedUrl: "https://storage.googleapis.com/test-bucket/test-object?X-Goog-Algorithm=GOOG4-RSA-SHA256&X-Goog-Credential=test-iam-credentials%40dummy-project-id.iam.gserviceaccount.com%2F20190201%2Fauto%2Fstorage%2Fgoog4_request&X-Goog-Date=20190201T090000Z&X-Goog-Expires=10&X-Goog-SignedHeaders=host&X-Goog-Signature=95e6a13d43a1d1962e667f17397f2b80ac9bdd1669210d5e08e0135df9dff4e56113485dbe429ca2266487b9d1796ebdee2d7cf682a6ef3bb9fbb4c351686fba90d7b621cf1c4eb1fdf126460dd25fa0837dfdde0a9fd98662ce60844c458448fb2b352c203d9969cb74efa4bdb742287744a4f2308afa4af0e0773f55e32e92973619249214b97283b2daa14195244444e33f938138d1e5f561088ce8011f4986dda33a556412594db7c12fc40e1ff3f1bedeb7a42f5bcda0b9567f17f65855f65071fabb88ea12371877f3f77f10e1466fff6ff6973b74a933322ff0949ce357e20abe96c3dd5cfab42c9c83e740a4d32b9e11e146f0eb3404d2e975896f74",
	},

	{
		FileName: "list-objects",
		Description: "List Objects",
		Bucket: "test-bucket",
		Object: "",
		Method: "GET",
		Expiration: 10,
		Timestamp: "20190201T090000Z",
		ExpectedUrl: "https://storage.googleapis.com/test-bucket/?X-Goog-Algorithm=GOOG4-RSA-SHA256&X-Goog-Credential=test-iam-credentials%40dummy-project-id.iam.gserviceaccount.com%2F20190201%2Fauto%2Fstorage%2Fgoog4_request&X-Goog-Date=20190201T090000Z&X-Goog-Expires=10&X-Goog-SignedHeaders=host&X-Goog-Signature=2a1d342f11ddf0c90c669b9ba89ab5099f94049a86351cacbc85845fd5a8b31e1f9c8d484926c19fbd6930da6c8d3049ca8ebcfeefb7b02e53137755d36f97baab479414528b2802f10d94541facb888edf886d91ba124e60cb3801464f61aadc575fc921c99cf8c52e281f7bc0d3e740f529201c469c8e52775b6433687e0c0dca1c6b874614c3c3d09599be1e192c40ad6827416e387bf6e88a5f501f1d8225bce498d134599d0dfe30c9c833c244d3f90cf9595b9f8175658b788ee5c4a90b575fde5e83c645772250c7098373ca754b39d0fc1ebca2f50261a015931541c9827920eba67a1c41613853a1bd23299a1f9f5d583c0feb05ea2f792ba390d27",
	},
}

func main() {
	fmt.Println("Generating")
	defer func() { fmt.Println("Generating... done!") }()

	m := jsonpb.Marshaler{
		Indent: "\t",
	}

	for _, t := range testcases {
		f, err := os.Create(fmt.Sprintf("testdata/%s.json", t.FileName))
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if err := m.Marshal(f, &t); err != nil {
			panic(err)
		}
	}
}
