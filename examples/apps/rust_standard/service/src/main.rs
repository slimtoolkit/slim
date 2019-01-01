extern crate actix;
extern crate actix_web;
extern crate bytes;
extern crate env_logger;
extern crate futures;
extern crate serde_json;
#[macro_use]
extern crate serde_derive;
#[macro_use]
extern crate json;
extern crate rustc_version_runtime;
extern crate semver;

use rustc_version_runtime::version;
use semver::Version;

use actix_web::{
    error, http, middleware, server, App, AsyncResponder, Error, HttpMessage,
    HttpRequest, HttpResponse, Json,
};

#[derive(Debug, Serialize, Deserialize)]
struct Response {
    status: String,
    data: String,
    service: String,
    version: String
}

fn on_call(
    req: &HttpRequest,
) -> HttpResponse
{
    let r = Response
    {
        status: "success".to_string(), 
        data: "yes!".to_string(),
        service: "rust".to_string(), 
        version: format!("{}.{}",version().major,version().minor)
    };

    HttpResponse::Ok().json(r)
}

fn main() 
{
    ::std::env::set_var("RUST_LOG", "actix_web=info");
    env_logger::init();
    let sys = actix::System::new("rust-service");

    server::new(|| 
    {
        App::new()
            .middleware(middleware::Logger::default())
            .resource("/", |r| r.method(http::Method::GET).f(on_call))
    }).bind("0.0.0.0:15000")
        .unwrap()
        .shutdown_timeout(1)
        .start();

    println!("Started Rust service on port 15000...");
    let _ = sys.run();
}
