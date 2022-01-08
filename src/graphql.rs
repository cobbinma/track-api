use juniper::{graphql_object, EmptySubscription, FieldResult};
use uuid::Uuid;

use crate::{repositories::cache::Cache, route::NewRoute, route::Route};

#[derive(Clone)]
pub struct State {
    pub repository: Cache,
}

impl juniper::Context for State {}

pub struct Query;

#[graphql_object(
    context = State,
)]
impl Query {
    async fn route(ctx: &State, id: Uuid) -> FieldResult<Route> {
        ctx.repository.get_route(id).await.map_err(From::from)
    }
}

pub struct Mutation;

#[graphql_object(
  context = State,
)]
impl Mutation {
    async fn create_route(ctx: &State, new_route: NewRoute) -> FieldResult<Route> {
        ctx.repository
            .create_route(new_route)
            .await
            .map_err(From::from)
    }
}

pub type Schema = juniper::RootNode<'static, Query, Mutation, EmptySubscription<State>>;
