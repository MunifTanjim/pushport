# Changelog

## [0.0.2](https://github.com/MunifTanjim/pushport/compare/0.0.1...0.0.2) (2026-09-11)


### Features

* **console:** add CSP and security headers ([e013a34](https://github.com/MunifTanjim/pushport/commit/e013a3491f1a20c56669e8cb05c33c0fc5ad618f))
* **fcm:** derive project id from service-account json ([cbeefef](https://github.com/MunifTanjim/pushport/commit/cbeefef4fa4af32200421171a074bf7d9deee521))
* **server:** redirect root to docs site ([b38749f](https://github.com/MunifTanjim/pushport/commit/b38749f3fd8cda8b4b2acc742976ec5458fc848c))


### Bug Fixes

* **cli:** exit non-zero when the server crashes ([98b7e01](https://github.com/MunifTanjim/pushport/commit/98b7e01e797b30ea590f82b0f5d0a4330fbcc7cd))
* **server:** bound PUSHPORT_PUSH_ENDPOINT_TTL to the client TTL window ([17565ee](https://github.com/MunifTanjim/pushport/commit/17565ee63e8fd0ff7264e7c5607f1940a425a567))
* **server:** charge push quota only after the body is received ([1380719](https://github.com/MunifTanjim/pushport/commit/1380719f9f7d2ef7dd15af2d446fcfece19ced4c))
* **server:** rate-limit the public register page per IP ([802803b](https://github.com/MunifTanjim/pushport/commit/802803beae8ad8511db0e38e8d671cce33984821))
* **server:** reject blank instance label on the authed path too ([e5986e7](https://github.com/MunifTanjim/pushport/commit/e5986e75bc093fbb58bd8fcf083e1181b47fc6d1))
* **server:** reject unsupported Content-Encoding with 400 ([5ca2135](https://github.com/MunifTanjim/pushport/commit/5ca2135f2b4d8cb57c1ebe48945793b1de28cce6))
* **server:** return 500 (not 401) on DB error during instance auth ([9e82e17](https://github.com/MunifTanjim/pushport/commit/9e82e1793ca6bfa2abed2a7bd3068f6d2828de75))
* **server:** stop double-counting total pushes in /stats ([910d275](https://github.com/MunifTanjim/pushport/commit/910d275ca686ea5c9a7336c88b5edd31682f7321))
* **server:** stop retrying when Retry-After exceeds budget ([24994c5](https://github.com/MunifTanjim/pushport/commit/24994c50e99f4f32f64ef6c6e5a2974e076577ad))
* **server:** uniform register-page response for unknown vs private apps ([f9d6825](https://github.com/MunifTanjim/pushport/commit/f9d682501f83994535c894f2886a5ceadfcb25d8))

## 0.0.1 (2026-09-06)


### Features

* **server:** push notification relay server and management CLI ([419705c](https://github.com/MunifTanjim/pushport/commit/419705c07230dbb8462e17ed242713472945578f))
