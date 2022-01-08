use juniper::{
    http::{graphiql, GraphQLRequest},
    EmptySubscription,
};
use lazy_static::lazy_static;
use repositories::cache::Cache;
use tide::{http::mime, Body, Redirect, Request, Response, Server, StatusCode};

mod graphql;
mod repositories;
mod route;

lazy_static! {
    static ref SCHEMA: graphql::Schema = graphql::Schema::new(
        graphql::Query {},
        graphql::Mutation {},
        EmptySubscription::new()
    );
}

async fn handle_graphql(mut request: Request<graphql::State>) -> tide::Result {
    let query: GraphQLRequest = request.body_json().await?;
    let response = query.execute(&SCHEMA, request.state()).await;
    let status = if response.is_ok() {
        StatusCode::Ok
    } else {
        StatusCode::BadRequest
    };

    Ok(Response::builder(status)
        .body(Body::from_json(&response)?)
        .build())
}

async fn handle_graphiql(_: Request<graphql::State>) -> tide::Result<impl Into<Response>> {
    Ok(Response::builder(200)
        .body(graphiql::graphiql_source("/graphql", None))
        .content_type(mime::HTML))
}

#[async_std::main]
async fn main() -> std::io::Result<()> {
    tide::log::start();
    let mut app = Server::with_state(graphql::State {
        repository: Cache::new(),
    });
    app.at("/").get(Redirect::permanent("/graphiql"));
    app.at("/graphql").post(handle_graphql);
    app.at("/graphiql").get(handle_graphiql);
    app.listen("0.0.0.0:8080").await?;

    Ok(())
}
