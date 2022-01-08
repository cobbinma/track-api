use juniper::{GraphQLEnum, GraphQLInputObject, GraphQLObject};
use uuid::Uuid;

#[derive(GraphQLEnum, Clone)]
pub enum RouteStatus {
    Active,
    Finished,
}

#[derive(GraphQLInputObject)]
#[graphql(description = "A new route")]
pub struct NewRoute {
    pub user_id: Uuid,
}

#[derive(GraphQLObject, Clone)]
#[graphql(description = "A route")]
pub struct Route {
    pub id: Uuid,
    pub user_id: Uuid,
    pub status: RouteStatus,
}
